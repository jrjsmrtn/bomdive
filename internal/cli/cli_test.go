package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func fx(name string) string {
	return filepath.Join("..", "..", "testdata", name+".cdx.json")
}

// run drives the CLI in-process. No subprocess, no os.Stdout capture — Run takes
// its streams, which is why the flags had to stop being package globals.
func run(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = Run("test", args, &o, &e)
	return code, o.String(), e.String()
}

func TestLsRendersComponents(t *testing.T) {
	code, out, errOut := run("ls", fx("diamond"))
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	for _, want := range []string{"app@1.0.0", "shared@1.0.0", "coverage:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTreeRendersGlyphs(t *testing.T) {
	code, out, _ := run("tree", fx("diamond"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "└── ") || !strings.Contains(out, "already shown") {
		t.Errorf("tree output not wired through:\n%s", out)
	}
}

// The flag has to reach the renderer, not merely parse.
func TestJSONFlagSwitchesSurface(t *testing.T) {
	code, out, _ := run("tree", "--json", fx("diamond"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json did not produce JSON: %v\n%s", err, out)
	}
	if _, ok := v["coverage"]; !ok {
		t.Error("JSON surface reached but coverage missing")
	}
}

func TestLongFlagReachesTheRenderer(t *testing.T) {
	_, short, _ := run("ls", fx("obom-categories"))
	_, long, _ := run("ls", "-l", fx("obom-categories"))
	if short == long {
		t.Error("-l changed nothing; the flag is not wired to the renderer")
	}
	if !strings.Contains(long, "[launchd_services]") {
		t.Errorf("long format missing the category column:\n%s", long)
	}
}

func TestByCategoryFlagIsWired(t *testing.T) {
	code, out, _ := run("ls", "--by-category", fx("obom-categories"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "launchd_services/") {
		t.Errorf("--by-category not wired:\n%s", out)
	}
}

func TestTypeFilterFlagIsWired(t *testing.T) {
	_, all, _ := run("ls", fx("hbom-depth1"))
	_, fw, _ := run("ls", "--type", "firmware", fx("hbom-depth1"))
	if len(strings.Split(fw, "\n")) >= len(strings.Split(all, "\n")) {
		t.Error("--type did not narrow the listing")
	}
}

func TestLevelFlagIsWired(t *testing.T) {
	_, deep, _ := run("tree", fx("cycle-deep"))
	_, shallow, _ := run("tree", "-L", "1", fx("cycle-deep"))
	if strings.Count(shallow, "\n") >= strings.Count(deep, "\n") {
		t.Error("-L did not cap depth")
	}
}

func TestReverseFlagIsWired(t *testing.T) {
	ref := "pkg:generic/shared@1.0.0"
	_, fwd, _ := run("ls", fx("diamond"), ref)
	_, rev, _ := run("ls", "-r", fx("diamond"), ref)
	if fwd == rev {
		t.Error("-r produced the same listing as forward; the flag is not wired")
	}
	if !strings.Contains(rev, "a@1.0.0") || !strings.Contains(rev, "b@1.0.0") {
		t.Errorf("reverse listing does not show dependents:\n%s", rev)
	}
}

// Two sequential runs with different flags must produce different output.
//
// ⚠ NOT proof that package-global flags would leak. Mutation testing showed they
// would not: cobra re-registers each flag with its default on every Run, so a
// shared app struct is reset anyway. Keeping this as a general guard against
// unreset state, and NOT claiming it catches a bug that was never demonstrated.
// The struct exists for design reasons — no shared mutable package state, and
// streams that a test can supply — not because a leak was observed.
func TestSequentialRunsDoNotShareFlagState(t *testing.T) {
	if _, jsonOut, _ := run("ls", "--json", fx("diamond")); !strings.HasPrefix(jsonOut, "{") {
		t.Fatal("first invocation did not emit JSON")
	}
	_, textOut, _ := run("ls", fx("diamond"))
	if strings.HasPrefix(textOut, "{") {
		t.Error("second invocation inherited --json: flag state leaked between runs")
	}
}

func TestErrorsExitNonZeroAndSayWhy(t *testing.T) {
	for name, args := range map[string][]string{
		"missing file":  {"tree", "/nonexistent.json"},
		"no arguments":  {"ls"},
		"too many args": {"ls", fx("diamond"), "a", "b"},
		"unknown flag":  {"ls", "--nope", fx("diamond")},
		"unknown cmd":   {"frobnicate", fx("diamond")},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := run(args...)
			if code == 0 {
				t.Errorf("exit=0 on %q; stdout=%q", name, out)
			}
			if errOut == "" {
				t.Error("failed silently: nothing on stderr")
			}
		})
	}
}

// Diagnostics must not land on stdout, or they corrupt a pipeline reading JSON.
func TestErrorsGoToStderrNotStdout(t *testing.T) {
	_, out, errOut := run("tree", "--json", "/nonexistent.json")
	if out != "" {
		t.Errorf("stdout polluted on error: %q", out)
	}
	if !strings.Contains(errOut, "lsxbom:") {
		t.Errorf("stderr lacks the error: %q", errOut)
	}
}

func TestVersionIsReported(t *testing.T) {
	code, out, _ := run("--version")
	if code != 0 || !strings.Contains(out, "test") {
		t.Errorf("exit=%d out=%q", code, out)
	}
}

func TestHelpNamesWhatTheToolIsNot(t *testing.T) {
	_, out, _ := run("--help")
	if !strings.Contains(out, "not a BOM generator") {
		t.Errorf("help omits the scope boundary:\n%s", out)
	}
}
