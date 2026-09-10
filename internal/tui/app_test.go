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
	for _, pr := range presses {
		sim.InjectKey(pr.k, pr.r, tcell.ModNone)
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(60 * time.Millisecond)
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
	lines := strings.SplitN(screen, "\n", 3)
	if len(lines) < 2 {
		return screen
	}
	return lines[0] + "\n" + lines[1]
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
	for _, want := range []string{"quit", "filter", "descend"} {
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
