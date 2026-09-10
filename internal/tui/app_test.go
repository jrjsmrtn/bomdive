package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

// screenText reads back every cell the app actually painted. This is the only way
// to know a view renders rather than merely compiles.
func screenText(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := cells[y*w+x].Runes
			if len(r) > 0 && r[0] != 0 {
				b.WriteRune(r[0])
			} else {
				b.WriteRune(' ')
			}
		}
		b.WriteRune('\n')
	}
	return b.String()
}

// key is one keypress: either a special key, or a rune.
type key struct {
	k tcell.Key
	r rune
}

func sp(k tcell.Key) key { return key{k: k} }
func ru(r rune) key      { return key{k: tcell.KeyRune, r: r} }

// drive runs the view on a simulation screen, sends keys, and returns what is drawn.
func drive(t *testing.T, fixture string, keys ...tcell.Key) string {
	t.Helper()
	presses := make([]key, 0, len(keys))
	for _, k := range keys {
		presses = append(presses, sp(k))
	}
	return driveKeys(t, fixture, presses...)
}

// driveKeys runs the view THROUGH run(), the real entry point, and injects keys
// through the simulation screen exactly as a terminal would deliver them.
//
// An earlier version hand-assembled the ui and wired the app itself, which tested
// around the entry point rather than through it — run() sat at 0% coverage while
// every behaviour appeared tested. Going through run() also exercises the
// before-draw hook that computes pane width, which is where two real bugs lived.
func driveKeys(t *testing.T, fixture string, presses ...key) string {
	t.Helper()
	g, err := bom.Load(filepath.Join("..", "..", "testdata", fixture+".cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(120, 30)

	done := make(chan error, 1)
	go func() { done <- run(g, fixture, sim) }()

	waitFor(t, sim, "coverage") // the app has painted at least once
	waitSettled(t, sim)         // ...and has stopped repainting
	for _, pr := range presses {
		sim.InjectKey(pr.k, pr.r, tcell.ModNone)
		time.Sleep(25 * time.Millisecond)
	}
	waitSettled(t, sim)
	out := screenText(sim)

	// Escape first: while the filter field has focus, 'q' is TEXT and not a command.
	// That is correct behaviour — discovered by this teardown failing — so the
	// teardown leaves filter mode before quitting. The screen was captured above,
	// so the assertion still sees the filtered state.
	sim.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	time.Sleep(25 * time.Millisecond)
	sim.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // the real quit path
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("app did not stop on q")
	}
	return out
}

// waitSettled blocks until two consecutive reads of the screen agree.
//
// The view repaints itself once geometry becomes known — pane width and detail
// height are only real after the first layout — so a capture taken too early
// differs from one taken after an unrelated keypress. Comparing those two made an
// UNBOUND key look as though it changed the view. Waiting for the screen to stop
// moving removes a whole class of false failure.
func waitSettled(t *testing.T, sim tcell.SimulationScreen) {
	t.Helper()
	prev := ""
	for i := 0; i < 60; i++ {
		time.Sleep(20 * time.Millisecond)
		cur := screenText(sim)
		if cur == prev && cur != "" {
			return
		}
		prev = cur
	}
	t.Fatal("screen never settled")
}

func waitFor(t *testing.T, sim tcell.SimulationScreen, want string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if strings.Contains(screenText(sim), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("screen never showed %q:\n%s", want, screenText(sim))
}

func TestViewPaintsColumnsAndDetail(t *testing.T) {
	out := drive(t, "diamond")
	for _, want := range []string{"app", "detail", "coverage", "roots"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen missing %q:\n%s", want, out)
		}
	}
}

// ADR-0004 makes coverage a guarantee across surfaces. A TUI is a third surface.
func TestCoverageIsOnScreen(t *testing.T) {
	out := drive(t, "partial-coverage")
	if !strings.Contains(out, "coverage") {
		t.Fatalf("no coverage on screen:\n%s", out)
	}
	if !strings.Contains(out, "3/10") {
		t.Errorf("coverage figures not shown:\n%s", out)
	}
	if !strings.Contains(out, "partial") {
		t.Error("a partial graph is not flagged on screen")
	}
}

// ADR-0007: forward and reverse are visually identical, so the view must say which.
// header returns just the top two lines. Scoping matters here: "dependencies" and
// "dependents" are ALSO detail-pane keys, so searching the whole screen for them
// passes even when the header says nothing — which is exactly how two planted
// defects escaped an earlier version of this test.
func header(screen string) string {
	lines := strings.SplitN(screen, "\n", 4)
	if len(lines) < 3 {
		return screen
	}
	// Three lines: the identity, the path, and the pane title row that carries the
	// detail pane's scroll indicator.
	return lines[0] + "\n" + lines[1] + "\n" + lines[2]
}

func TestDirectionIsStatedAndFlips(t *testing.T) {
	fwd := header(drive(t, "diamond"))
	if !strings.Contains(fwd, "showing: dependencies") {
		t.Errorf("forward direction not stated in the header: %q", fwd)
	}
	rev := header(drive(t, "diamond", tcell.KeyRight, tcell.KeyTab))
	if !strings.Contains(rev, "showing: dependents") {
		t.Errorf("direction did not flip on Tab: %q", rev)
	}
	if strings.Contains(rev, "showing: dependencies") {
		t.Errorf("header still claims forward after flipping: %q", rev)
	}
}

func TestDescendingAppendsAColumnAndExtendsThePath(t *testing.T) {
	before := drive(t, "diamond")
	after := drive(t, "diamond", tcell.KeyRight)
	if strings.Count(after, ">") <= strings.Count(before, ">") {
		t.Errorf("path did not extend on descent:\n%s", after)
	}
}

// A graphless BOM must open somewhere navigable and say why.
func TestGraphlessBOMOpensOnCategoriesAndSaysSo(t *testing.T) {
	out := drive(t, "obom-categories")
	if !strings.Contains(out, "categories") {
		t.Errorf("did not open on categories:\n%s", out)
	}
	if !strings.Contains(out, "no dependency graph") {
		t.Errorf("screen does not explain why there is no tree:\n%s", out)
	}
}

func TestKeyHintsAreVisible(t *testing.T) {
	out := drive(t, "diamond")
	for _, want := range []string{"quit", "filter", "detail", "nav"} {
		if !strings.Contains(out, want) {
			t.Errorf("key hint %q missing; the view is undiscoverable:\n%s", want, out)
		}
	}
}

func TestEveryFixtureRendersWithoutPanicking(t *testing.T) {
	for _, f := range []string{
		"tree-simple", "diamond", "cycle-direct", "cycle-self", "rootless",
		"multi-root", "no-dependencies-empty", "obom-categories", "hbom-depth1",
		"purl-less", "dangling-ref",
	} {
		t.Run(f, func(t *testing.T) {
			if out := drive(t, f, tcell.KeyRight, tcell.KeyTab, tcell.KeyLeft); out == "" {
				t.Error("nothing painted")
			}
		})
	}
}

// A lone column must use the whole width available to it. Dividing by the maximum
// column count instead of the actual one truncated entries that fit comfortably.
func TestSingleColumnUsesTheFullWidth(t *testing.T) {
	out := drive(t, "obom-categories")
	for _, want := range []string{"certificates", "launchd_services", "listening_ports"} {
		if !strings.Contains(out, want) {
			t.Errorf("category %q truncated in a column with room for it:\n%s", want, out)
		}
	}
}

// vi keys are advertised nowhere on screen, so if they silently stop working
// nobody finds out. They must behave identically to the arrows.
func TestViKeysMatchTheArrows(t *testing.T) {
	arrows := drive(t, "diamond", tcell.KeyRight)
	vi := driveKeys(t, "diamond", ru('l'))
	if arrows != vi {
		t.Error("l does not match Right")
	}
	back := driveKeys(t, "diamond", ru('l'), ru('h'))
	start := drive(t, "diamond")
	if back != start {
		t.Error("h does not match Left")
	}
}

func TestJAndKMoveTheCursor(t *testing.T) {
	top := driveKeys(t, "diamond", ru('l'))
	down := driveKeys(t, "diamond", ru('l'), ru('j'))
	if top == down {
		t.Error("j did not move the cursor")
	}
	if backUp := driveKeys(t, "diamond", ru('l'), ru('j'), ru('k')); backUp != top {
		t.Error("k did not return the cursor")
	}
}

// The detail pane must follow the cursor, or it describes the wrong component.
func TestDetailFollowsTheCursor(t *testing.T) {
	first := driveKeys(t, "diamond", ru('l'))
	second := driveKeys(t, "diamond", ru('l'), ru('j'))
	if !strings.Contains(first, "pkg:generic/a@1.0.0") {
		t.Errorf("detail does not describe the first entry:\n%s", first)
	}
	if !strings.Contains(second, "pkg:generic/b@1.0.0") {
		t.Errorf("detail did not follow the cursor:\n%s", second)
	}
}

func TestFilterModeNarrowsTheColumn(t *testing.T) {
	out := driveKeys(t, "obom-categories", ru('/'), ru('c'), ru('e'), ru('r'))
	if !strings.Contains(out, "filter") {
		t.Errorf("filter prompt not shown:\n%s", out)
	}
	if !strings.Contains(out, "certificates") {
		t.Errorf("filtered column lost its match:\n%s", out)
	}
	if strings.Contains(out, "systemd_units") {
		t.Errorf("filter did not narrow the column:\n%s", out)
	}
}

func TestEscapeClearsTheFilter(t *testing.T) {
	out := driveKeys(t, "obom-categories", ru('/'), ru('c'), ru('e'), ru('r'), sp(tcell.KeyEscape))
	if !strings.Contains(out, "systemd_units") {
		t.Errorf("Escape did not restore the column:\n%s", out)
	}
}

func TestUnhandledRuneIsPassedThroughNotSwallowed(t *testing.T) {
	// 'z' is not bound; the view must not treat it as a command or redraw oddly.
	plain := drive(t, "diamond")
	after := driveKeys(t, "diamond", ru('z'))
	if plain != after {
		t.Error("an unbound key changed the view")
	}
}

// A pane cut off with no way down, and no sign there IS a down, looks complete.
// That is the same failure the coverage line exists to prevent, one pane over.
func TestDetailPaneShowsAnOverflowIndicator(t *testing.T) {
	out := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight)
	if !strings.Contains(out, "detail") {
		t.Fatalf("no detail pane:\n%s", out)
	}
	if !strings.Contains(out, "of") || !strings.Contains(out, "↓") {
		t.Errorf("detail pane does not say more content exists:\n%s", header(out))
	}
}

func TestDetailPaneScrolls(t *testing.T) {
	top := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight)
	down := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight, tcell.KeyPgDn)
	if top == down {
		t.Error("PgDn did not scroll the detail pane")
	}
	if !strings.Contains(down, "↑") {
		t.Errorf("scrolled pane does not indicate content above:\n%s", header(down))
	}
	back := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight, tcell.KeyPgDn, tcell.KeyPgUp)
	if back != top {
		t.Error("PgUp did not return to the top")
	}
}

func TestDetailScrollResetsWhenTheSelectionChanges(t *testing.T) {
	// Scroll, then move: an offset carried across components would show the middle
	// of one record under another's header.
	moved := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight, tcell.KeyPgDn, tcell.KeyDown)
	fresh := drive(t, "many-properties", tcell.KeyDown, tcell.KeyRight, tcell.KeyDown)
	if moved != fresh {
		t.Error("detail scroll survived a selection change")
	}
}

func TestDetailScrollIsClamped(t *testing.T) {
	// Scrolling far past the end must not leave a blank pane that reads as "nothing here".
	keys := []tcell.Key{tcell.KeyDown, tcell.KeyRight}
	for i := 0; i < 20; i++ {
		keys = append(keys, tcell.KeyPgDn)
	}
	out := drive(t, "many-properties", keys...)
	// Clamping means the LAST page is shown, so the header fields have scrolled off
	// — asserting on those tested the wrong thing. What matters is that content is
	// still visible and the pane says it is at the end.
	if !strings.Contains(out, "field_3") {
		t.Errorf("scrolled past the end into blankness:\n%s", out)
	}
	if strings.Contains(header(out), "↓") {
		t.Errorf("pane still claims more content below after clamping:\n%s", header(out))
	}
	if !strings.Contains(header(out), "↑") {
		t.Errorf("pane does not indicate content above:\n%s", header(out))
	}
}

// A component with no name rendered as an unselectable-looking blank row. Seen in
// a real HBOM; bom-ref is the only field guaranteed to be present.
func TestUnnamedComponentFallsBackToItsRef(t *testing.T) {
	// The "alf" category holds a named and an unnamed component. The unnamed one
	// must show SOMETHING selectable rather than an empty row.
	out := drive(t, "many-properties", tcell.KeyRight)
	if !strings.Contains(out, "unnamed") {
		t.Errorf("a component with no name rendered as a blank row:\n%s", out)
	}
}

// Without clamping, over-scrolling leaves the offset far past the end, so PgUp has
// to undo all of it before anything moves. That is observable, which is why the
// clamp is not merely defensive.
func TestOverScrollingThenPagingUpMovesImmediately(t *testing.T) {
	keys := []tcell.Key{tcell.KeyDown, tcell.KeyRight}
	for i := 0; i < 20; i++ {
		keys = append(keys, tcell.KeyPgDn)
	}
	atEnd := drive(t, "many-properties", keys...)
	up := drive(t, "many-properties", append(append([]tcell.Key{}, keys...), tcell.KeyPgUp)...)
	if atEnd == up {
		t.Error("PgUp after over-scrolling did nothing; the offset was not clamped")
	}
	if !strings.Contains(header(up), "↓") {
		t.Errorf("paging up from the end does not show content below:\n%s", header(up))
	}
}

// Few fields, long values: the shape where a two-rows-per-field estimate is wrong.
//
// This is what dogfooding an OBOM exposed. The pane wraps, so five fields with
// 240-character values occupy far more than ten rows — but the estimate said ten,
// clamped the scroll to zero, and the title claimed everything was visible.
func TestDetailScrollsWhenContentOverflowsByWrappingAlone(t *testing.T) {
	top := drive(t, "long-values", tcell.KeyRight)
	if !strings.Contains(header(top), "of") {
		t.Fatalf("pane does not report overflow on wrapped content:\n%s", header(top))
	}
	down := drive(t, "long-values", tcell.KeyRight, tcell.KeyPgDn)
	if top == down {
		t.Error("PgDn did not scroll wrapped content")
	}
}

func TestWrappedRowsCountsWrapping(t *testing.T) {
	for _, tc := range []struct {
		s     string
		width int
		want  int
	}{
		{"", 10, 1},
		{"short", 10, 1},
		{"exactlyten", 10, 1},
		{"eleven chars", 10, 2},
		{"a very long value that needs several rows", 10, 5},
	} {
		if got := wrappedRows(tc.s, tc.width); got != tc.want {
			t.Errorf("wrappedRows(%q, %d) = %d, want %d", tc.s, tc.width, got, tc.want)
		}
	}
}

// The status bar must not call a silent document "partial".
//
// It did: the yellow branch fired only on DeclaresNoGraph — `dependencies` present
// and empty — so a BOM with no `dependencies` field at all fell through to the red
// "partial" used for a declared graph that genuinely misses components. Reported
// from dogfooding a syft fixture, where 0/1 (0%) partial reads as a broken tool.
func TestStatusLabelsEachGraphStateDistinctly(t *testing.T) {
	want := map[string]string{
		"tree-simple":            "",
		"partial-coverage":       "partial",
		"no-dependencies-empty":  "no dependency graph",
		"no-dependencies-absent": "relations undeclared",
	}
	for name, label := range want {
		g, err := bom.Load(filepath.Join("..", "..", "testdata", name+".cdx.json"))
		if err != nil {
			t.Fatal(err)
		}
		got := newUI(g, name).statusText()
		if label == "" {
			for _, other := range want {
				if other != "" && strings.Contains(got, other) {
					t.Errorf("%s: complete graph labelled %q: %s", name, other, got)
				}
			}
			continue
		}
		if !strings.Contains(got, label) {
			t.Errorf("%s: status %q does not carry %q", name, got, label)
		}
		// The label must be its OWN state's, not a neighbour's.
		for other, otherLabel := range want {
			if other == name || otherLabel == "" || otherLabel == label {
				continue
			}
			if strings.Contains(got, otherLabel) {
				t.Errorf("%s: status carries %q, which belongs to %s: %s",
					name, otherLabel, other, got)
			}
		}
	}
}

// `?` must explain the word the status bar had no room to explain, for THIS
// document — and it must go away again.
func TestExplainOverlayExplainsTheStatusLabel(t *testing.T) {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "no-dependencies-absent.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := g.Coverage().Explain()

	opened := driveKeys(t, "no-dependencies-absent", ru('?'))
	// The overlay wraps, so compare on collapsed whitespace rather than on a
	// line-for-line match the pane width decides.
	if !strings.Contains(flatten(opened), flatten(want)) {
		t.Errorf("? does not explain the status label.\nwant: %s\ngot:\n%s", want, opened)
	}

	closed := driveKeys(t, "no-dependencies-absent", ru('?'), ru('?'))
	if strings.Contains(flatten(closed), flatten(want)) {
		t.Errorf("? did not close the overlay:\n%s", closed)
	}
	if !strings.Contains(closed, "coverage") {
		t.Errorf("the view did not come back after closing the overlay:\n%s", closed)
	}
}

// Esc closes it too, and does NOT fall through to clearing the column filter.
func TestEscapeClosesTheHelpOverlay(t *testing.T) {
	out := driveKeys(t, "tree-simple", ru('?'), sp(tcell.KeyEscape))
	if strings.Contains(out, "what this means") {
		t.Errorf("Esc left the overlay open:\n%s", out)
	}
}

// The two overlays answer different questions and must stay separate: `?` explains
// THIS document, `H` lists the keys. Merging them buries the contextual half under
// a key table, which is why they were split.
func TestExplanationAndHelpAreSeparateOverlays(t *testing.T) {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "partial-coverage.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	explain := flatten(driveKeys(t, "partial-coverage", ru('?')))
	help := flatten(driveKeys(t, "partial-coverage", ru('H')))

	if !strings.Contains(explain, flatten(g.Coverage().Explain())) {
		t.Errorf("? does not explain the document:\n%s", explain)
	}
	if strings.Contains(explain, "flip between depends-on") {
		t.Error("? carries the key table; the explanation is buried under it")
	}
	if !strings.Contains(help, "flip between depends-on") {
		t.Errorf("H does not list the keys:\n%s", help)
	}
	if strings.Contains(help, flatten(g.Coverage().Explain())) {
		t.Error("H carries the document explanation; the two overlays are the same again")
	}
}

// `h` stays Left. It is vi navigation, and this tool's premise is that ls/tree
// muscle memory carries over — which is why help is H and not h.
func TestLowercaseHStillNavigatesLeft(t *testing.T) {
	descended := driveKeys(t, "tree-simple", ru('l'))
	if !strings.Contains(descended, ">") {
		t.Fatalf("l did not descend, so this test proves nothing:\n%s", descended)
	}
	back := driveKeys(t, "tree-simple", ru('l'), ru('h'))
	for _, title := range []string{"what this means", "keys —"} {
		if strings.Contains(back, title) {
			t.Errorf("h opened the %q overlay instead of going back:\n%s", title, back)
		}
	}
	if strings.Contains(back, ">") {
		t.Errorf("h did not go back a column:\n%s", back)
	}
}

// flatten reads a wrapped pane back as one string.
//
// screenText returns whole SCREEN rows, so a paragraph inside a bordered pane comes
// back interleaved with border glyphs and with whatever the underlying page is
// painting either side of it. Dropping the box-drawing runes and collapsing runs of
// whitespace rejoins the wrapped lines into the sentence that was written, so an
// assertion does not have to guess where the pane chose to break.
func flatten(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("│║╔╗╚╝┌┐└┘├┤┬┴┼─═", r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
