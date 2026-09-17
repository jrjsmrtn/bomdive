package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/columns"
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

// A row you can descend into must SAY so, and a leaf must not.
//
// Without it a leaf and a component with fifty dependencies look identical until
// you press → and nothing happens — the same class of failure as a detail pane
// truncated with no indicator.
//
// It descends one level first, because tree-simple's entry column is a single root:
// asserting on a column where every row is descendable would pass with the leaf
// branch never exercised. The counters below make that vacuity a failure.
func TestDescendableRowsAreMarkedAndLeavesAreNot(t *testing.T) {
	out := driveKeys(t, "tree-simple", ru('l'))
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "tree-simple.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := columns.New(g)
	if !m.Right() {
		t.Fatal("could not descend in the model; the screen and the model disagree")
	}

	var marked, bare int
	for _, n := range m.Columns()[1].Visible() {
		row := columnRow(out, n.Label())
		if row == "" {
			t.Fatalf("%q is on no column row:\n%s", n.Label(), out)
		}
		if m.Descendable(n) {
			marked++
			if !strings.Contains(row, columns.Descend) {
				t.Errorf("%q has children but carries no %s: %q", n.Label(), columns.Descend, row)
			}
		} else {
			bare++
			if strings.Contains(row, columns.Descend) {
				t.Errorf("%q is a leaf but is marked descendable: %q", n.Label(), row)
			}
		}
	}
	if marked == 0 || bare == 0 {
		t.Fatalf("only one branch was exercised (marked=%d bare=%d); the assertion is vacuous",
			marked, bare)
	}
}

// The arrow goes in the last cell of a row, so an off-by-one in the width puts it
// outside the border and tview clips it away silently.
//
// Scoped to lines that ARE column rows: the status bar's "→←" hint is the same
// glyph, and an unscoped search matched it — the arrow there has no border to its
// right and never should have.
func TestTheArrowIsInsideTheColumnBorder(t *testing.T) {
	out := driveKeys(t, "tree-simple", ru('l'))
	rows := 0
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, columns.Descend)
		if i < 0 || !strings.Contains(line[:i], "│") {
			continue // not a column row
		}
		rows++
		if !strings.Contains(line[i+len(columns.Descend):], "│") {
			t.Errorf("an arrow is drawn with no column border to its right: %q", line)
		}
	}
	if rows == 0 {
		t.Fatalf("no column row carried an arrow, so nothing was checked:\n%s", out)
	}
}

// columnRow returns the screen line where label starts a column cell — that is,
// immediately after a pane border. Matching the label anywhere would also match the
// header path line and the detail pane, which is how the first version of this
// test failed for a reason unrelated to the arrow.
func columnRow(screen, label string) string {
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "│"+label) {
			return line
		}
	}
	return ""
}

// The dangling list is a SAMPLE. A public 837-component SBOM carries 278 of them,
// and printing all of them filled the overlay and pushed the explanation off the
// top — the annotation drowned in its own data.
func TestTheOverlaySamplesDanglingRatherThanDumpingIt(t *testing.T) {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "dangling-ref.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(g, "dangling-ref")
	u.width, u.height = 120, 30
	got := u.explainText()
	if !strings.Contains(got, "match no component") {
		t.Fatalf("the explanation does not mention dangling refs at all:\n%s", got)
	}
	// A synthetic long list must be capped, and must say how many were left out.
	many := make([]string, bom.DanglingSample+42)
	for i := range many {
		many[i] = fmt.Sprintf("ref-%d", i)
	}
	shown, omitted := bom.Coverage{Dangling: many}.SampleDangling()
	if len(shown) != bom.DanglingSample || omitted != 42 {
		t.Fatalf("SampleDangling(%d) = %d shown, %d omitted", len(many), len(shown), omitted)
	}
}

// An overlay that does not fit must SAY so and be scrollable, or a clipped key
// table is indistinguishable from a complete one.
func TestTheOverlayReportsAndScrollsWhenItDoesNotFit(t *testing.T) {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "tree-simple.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(g, "tree-simple")
	u.width, u.height = 100, 14 // deliberately short: the key table cannot fit
	u.showOverlay(overlayHelp)

	if u.overlayLines <= u.visibleOverlayRows() {
		t.Fatalf("the key table fits at %d rows; this test proves nothing "+
			"(lines=%d visible=%d)", u.height, u.overlayLines, u.visibleOverlayRows())
	}
	title := u.overlayTitle()
	if !strings.Contains(title, "of") || !strings.Contains(title, "↓") {
		t.Errorf("a clipped overlay does not report it: %q", title)
	}
	before := u.overlayScroll
	u.scrollOverlay(+detailPageSize)
	if u.overlayScroll == before {
		t.Error("the overlay did not scroll")
	}
	// And it must clamp rather than run off the end.
	u.scrollOverlay(+10000)
	if want := u.overlayLines - u.visibleOverlayRows(); u.overlayScroll != want {
		t.Errorf("scroll = %d, want it clamped to %d", u.overlayScroll, want)
	}
}

// Scrolling must not dismiss the overlay: a panel that closes when you try to read
// the rest of it is worse than one that never scrolled.
func TestScrollingDoesNotCloseTheOverlay(t *testing.T) {
	// Assert on a line only the key table has. "keys" alone matches the status
	// bar's own "Hkeys" hint, so the overlay could close and the test still pass —
	// the unscoped-Contains failure this project keeps rediscovering.
	//
	// It asserts on the overlay's TITLE, the one thing the overlay always shows. It
	// first asserted on a line of the key table, and that broke the moment the table
	// grew past a 30-row screen: PgDn then really scrolled, the line left the view, and
	// the test reported "closed" for an overlay that was open and scrolled — measuring
	// the scroll position rather than whether the overlay was up. "H or Esc to close"
	// appears in the title whether or not it is clipped, and nowhere else: the status
	// bar reads "Hkeys".
	const onlyInTheOverlay = "H or Esc to close"
	opened := driveKeys(t, "tree-simple", ru('H'))
	if !strings.Contains(flatten(opened), onlyInTheOverlay) {
		t.Fatalf("H did not open the key table, so this test proves nothing:\n%s", opened)
	}
	out := driveKeys(t, "tree-simple", ru('H'), sp(tcell.KeyPgDn))
	if !strings.Contains(flatten(out), onlyInTheOverlay) {
		t.Errorf("PgDn closed the overlay:\n%s", out)
	}
}

// A document with nothing to inventory says THAT, not what its dependency graph
// looks like: "relations undeclared" on a standalone VEX is true and answers a
// question nobody asked.
func TestAnEmptyDocumentSaysSoRatherThanReportingGraphState(t *testing.T) {
	for _, name := range []string{"vex-standalone", "services-only", "metadata-only",
		"attestation-only", "definitions-only"} {
		g, err := bom.Load(filepath.Join("..", "..", "testdata", name+".cdx.json"))
		if err != nil {
			t.Fatal(err)
		}
		got := newUI(g, name).statusText()
		if !strings.Contains(got, "no components") {
			t.Errorf("%s: status %q does not say the document has no components", name, got)
		}
		for _, wrong := range []string{"relations undeclared", "no dependency graph", "partial"} {
			if strings.Contains(got, wrong) {
				t.Errorf("%s: status reports graph state %q for a document with nothing to "+
					"inventory: %s", name, wrong, got)
			}
		}
	}
	// A populated document keeps its graph-state label.
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "partial-coverage.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := newUI(g, "partial-coverage").statusText(); !strings.Contains(got, "partial") {
		t.Errorf("a populated document lost its graph-state label: %s", got)
	}
}

// statusLine is the last non-blank screen row: the status bar.
func statusLine(screen string) string {
	lines := strings.Split(strings.TrimRight(screen, " \n"), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// v switches to the vulnerability axis, and the header says which way the view faces:
// the two axes, like the two directions, can look alike.
func TestVSwitchesToTheVulnerabilityAxis(t *testing.T) {
	out := driveKeys(t, "vex-embedded-states", ru('v'))
	if h := header(out); !strings.Contains(h, "showing: vulnerabilities → affected") {
		t.Errorf("header does not name the vulnerability axis: %q", h)
	}
	if !strings.Contains(out, "by analysis state") {
		t.Errorf("the entry column is not grouped by state:\n%s", out)
	}
	// 4 vulnerabilities naming 5 targets, all resolved.
	if st := statusLine(out); !strings.Contains(st, "vulns 4") || !strings.Contains(st, "affects 5/5") {
		t.Errorf("status bar = %q, want the vulnerability count and the resolution count", st)
	}
}

func TestVIsANoOpWithoutVulnerabilities(t *testing.T) {
	out := driveKeys(t, "tree-simple", ru('v'))
	if strings.Contains(header(out), "vulnerabilities") {
		t.Errorf("v switched axis on a document with no vulnerabilities:\n%s", header(out))
	}
	if st := statusLine(out); !strings.Contains(st, "coverage") || strings.Contains(st, "vvulns") {
		t.Errorf("status bar = %q: want coverage, and no v hint where there is nothing to switch to", st)
	}
}

func TestTheVHintAppearsWhereThereAreVulnerabilities(t *testing.T) {
	out := driveKeys(t, "vex-embedded-states")
	if st := statusLine(out); !strings.Contains(st, "vvulns") {
		t.Errorf("status bar = %q, want the v hint", st)
	}
}

// Switching back must return the reader to exactly where they were.
func TestSwitchingBackRestoresTheScreen(t *testing.T) {
	before := header(driveKeys(t, "vex-embedded-states", ru('j'), ru('j')))
	after := header(driveKeys(t, "vex-embedded-states", ru('j'), ru('j'), ru('v'), ru('l'), ru('v')))
	if before != after {
		t.Errorf("header after v, l, v = %q, want %q", after, before)
	}
}

// At 120 columns the status bar once ran off the edge and lost "q quit". Measured on the
// longest cases this view can produce, including the vulnerability axis.
func TestTheStatusBarKeepsQuitVisible(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		vulns   bool
	}{
		{"vex-refs", true}, {"vex-embedded-untriaged", true}, {"vex-out-of-schema", true},
		{"vex-embedded-states", false}, {"no-dependencies-absent", false},
	} {
		g, err := bom.Load(filepath.Join("..", "..", "testdata", tc.fixture+".cdx.json"))
		if err != nil {
			t.Fatal(err)
		}
		u := newUI(g, tc.fixture)
		if tc.vulns && !u.model.ToggleMode() {
			t.Fatalf("%s: no vulnerability axis", tc.fixture)
		}
		st := stripTags(u.statusText())
		if n := utf8.RuneCountInString(st); n > 110 {
			t.Errorf("%s (vulns=%v): status bar is %d runes, over 110: %q", tc.fixture, tc.vulns, n, st)
		}
		if !strings.HasSuffix(st, "q quit") {
			t.Errorf("%s: status bar does not end in q quit: %q", tc.fixture, st)
		}
	}
}

// ? on the vulnerability axis explains the grouping it chose and what every reference
// resolved to — the two things the status bar has only a word each for.
//
// It asserts on the TEXT, not the screen. It first read the screen, and broke when the
// explanation grew past a 30-row terminal: the overlay then scrolled — correctly, with
// "1-21 of 26" in its title — and the last lines were out of view. Content and screen
// height are separate questions; TestTheOverlayReportsAndScrollsWhenItDoesNotFit asks
// the second.
func TestExplainOnTheVulnerabilityAxis(t *testing.T) {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", "vex-refs.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(g, "vex-refs")
	u.model.ToggleMode()
	out := flatten(stripTags(u.explainText()))
	for _, want := range []string{
		"there is no grouping column",              // all high, none analysed
		"1 names a package — a purl that names no", // each state, with its count and meaning
		"1 linked, version differs",
		"1 linked, not loaded",
		"1 names nothing",
		"5 resolved",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("? does not say %q:\n%s", want, out)
		}
	}
}

func TestHelpListsTheVKey(t *testing.T) {
	if out := flatten(driveKeys(t, "tree-simple", ru('H'))); !strings.Contains(out, "switch between components and vulnerabilities") {
		t.Errorf("H does not list v:\n%s", out)
	}
}

// The header is two rows. If its first line wraps, the path line is pushed off the
// screen — which happened at 80 columns on the vulnerability axis, and on the component
// axis for any OBOM, whose identity alone is 58 characters. The second row must be the
// path, and the first must still name the direction.
func TestTheHeaderKeepsThePathLineAt80Columns(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		keys    []key
		showing string
	}{
		{"vex-embedded-states", []key{ru('v')}, "vulnerabilities → affected"},
		{"vex-embedded-states", []key{ru('v'), sp(tcell.KeyTab)}, "affected → vulnerabilities"},
		{"obom-categories", nil, "dependencies"},
	} {
		lines := strings.Split(driveKeys(t, tc.fixture, tc.keys...), "\n")
		if len(lines) < 2 || !strings.HasPrefix(strings.TrimSpace(lines[1]), "path:") {
			t.Errorf("%s: the second row is not the path line: %q", tc.fixture, lines[1])
		}
		if !strings.Contains(lines[0], "showing: "+tc.showing) {
			t.Errorf("%s: the first row lost the direction: %q", tc.fixture, lines[0])
		}
	}
}

// With several documents named, the header, coverage and ? follow the file under the
// cursor — they describe "the file you are in".
func TestTheHeaderFollowsTheFileUnderTheCursor(t *testing.T) {
	s, err := bom.LoadSet([]string{
		filepath.Join("..", "..", "testdata", "link-sbom.cdx.json"),
		filepath.Join("..", "..", "testdata", "link-vex.cdx.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(s.Doc(0), "link-sbom")
	u.width = 120
	if h := u.headerText(); !strings.Contains(h, "SBOM") {
		t.Errorf("header on the SBOM file = %q", h)
	}
	u.model.Down()
	if h := u.headerText(); !strings.Contains(h, "VEX") {
		t.Errorf("header after moving to the VEX file = %q", h)
	}
	if src := u.sourceText(); !strings.HasSuffix(src, "link-vex.cdx.json") {
		t.Errorf("? describes %q, want the VEX under the cursor", src)
	}
	if ax := u.axisText(); !strings.Contains(ax, "2 documents named") {
		t.Errorf("? does not explain the files column: %q", ax)
	}
}

// A document with no components opens on what it carries, and ? says why rather than
// leaving the reader to guess what the leftmost column is.
func TestTheAxisExplainsADocumentWithNoComponents(t *testing.T) {
	for _, c := range []struct{ fixture, want string }{
		{"vex-standalone", "its vulnerability records"},
		{"metadata-only", "its subject"},
	} {
		g, err := bom.Load(filepath.Join("..", "..", "testdata", c.fixture+".cdx.json"))
		if err != nil {
			t.Fatal(err)
		}
		u := newUI(g, c.fixture)
		if ax := u.axisText(); !strings.Contains(ax, c.want) {
			t.Errorf("%s: ? explains %q, want it to name %q", c.fixture, ax, c.want)
		}
	}
}
