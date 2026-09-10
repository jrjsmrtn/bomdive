// Package steps holds the godog step definitions.
//
// The steps drive the CLI through cli.Run with in-memory streams — the same path
// the smoke tests use, and the same path a terminal uses. No subprocess, so a
// feature runs as fast as a unit test and a failure points at real code.
package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/jrjsmrtn/lsxbom/internal/cli"
)

type world struct {
	bomPath string
	args    []string
	stdout  string
	stderr  string
	code    int
}

func (w *world) reset() { *w = world{} }

func (w *world) theBOMFixture(name string) error {
	p := filepath.Join("..", "..", "testdata", name+".cdx.json")
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("no fixture %q: %w", name, err)
	}
	w.bomPath = p
	return nil
}

func (w *world) aBOMPathThatDoesNotExist() error {
	w.bomPath = filepath.Join(os.TempDir(), "lsxbom-no-such-bom.json")
	return nil
}

// run splits the quoted command from the feature into argv and appends the BOM
// path, so a scenario reads as the command a person would type.
func (w *world) iRun(cmd string) error {
	if w.bomPath == "" {
		return fmt.Errorf("no BOM given; the scenario needs a Given step")
	}
	args := append(strings.Fields(cmd), w.bomPath)
	return w.exec(args)
}

func (w *world) iRunFrom(cmd, ref string) error {
	args := append(strings.Fields(cmd), w.bomPath, ref)
	return w.exec(args)
}

func (w *world) exec(args []string) error {
	var out, errBuf bytes.Buffer
	w.code = cli.Run("bdd", args, &out, &errBuf)
	w.stdout, w.stderr = out.String(), errBuf.String()
	w.args = args
	return nil
}

func (w *world) theCommandFails() error {
	if w.code == 0 {
		return fmt.Errorf("expected a non-zero exit, got 0")
	}
	return nil
}

func (w *world) stdoutIsEmpty() error {
	if w.stdout != "" {
		return fmt.Errorf("stdout is not empty: %q", w.stdout)
	}
	return nil
}

// componentLines counts rendered entries, ignoring the header, notes and coverage.
func (w *world) componentLines() int {
	n := 0
	for _, l := range strings.Split(w.stdout, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "note:") || strings.HasPrefix(t, "coverage:") ||
			strings.HasPrefix(t, "warning:") || strings.HasPrefix(t, "from:") ||
			strings.Contains(t, "phases:") || strings.Contains(t, "undeclared") {
			continue
		}
		n++
	}
	return n
}

func (w *world) theOutputListsComponents(want int) error {
	if got := w.componentLines(); got != want {
		return fmt.Errorf("listed %d components, want %d:\n%s", got, want, w.stdout)
	}
	return nil
}

func (w *world) theOutputContains(s string) error {
	if !strings.Contains(w.stdout, s) {
		return fmt.Errorf("output does not contain %q:\n%s", s, w.stdout)
	}
	return nil
}

func (w *world) theOutputReportsCoverage() error { return w.theOutputContains("coverage:") }

func (w *world) theOutputReportsCoverageOf(in, of int) error {
	return w.theOutputContains(fmt.Sprintf("coverage: %d of %d", in, of))
}

func (w *world) theOutputMarksABackReference() error { return w.theOutputContains("already shown") }

func (w *world) theOutputMarksACycle() error { return w.theOutputContains("cycle") }

func (w *world) theOutputDoesNotMarkACycle() error {
	if strings.Contains(w.stdout, "cycle") {
		return fmt.Errorf("output marks a cycle where there is none:\n%s", w.stdout)
	}
	return nil
}

func (w *world) theOutputWarnsNotEveryComponentIsShown() error {
	return w.theOutputContains("does NOT show every component")
}

func (w *world) theOutputSaysRootsWereDerived() error { return w.theOutputContains("DERIVED") }

func (w *world) theOutputExplainsNoDependencyGraph() error {
	return w.theOutputContains("declares no dependency graph")
}

func (w *world) theOutputSuggestsListingByCategory() error {
	return w.theOutputContains("--by-category")
}

func (w *world) theOutputExplainsUnknownComponent() error {
	return w.theOutputContains("no component with bom-ref")
}

func (w *world) theOutputIsValidJSON() error {
	var v any
	if err := json.Unmarshal([]byte(w.stdout), &v); err != nil {
		return fmt.Errorf("not JSON: %w\n%s", err, w.stdout)
	}
	return nil
}

func (w *world) jsonField(path string) (any, error) {
	var v map[string]any
	if err := json.Unmarshal([]byte(w.stdout), &v); err != nil {
		return nil, err
	}
	var cur any = v
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not an object at %q", path, part)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("no field %q", path)
		}
	}
	return cur, nil
}

func (w *world) jsonFieldIsBool(path string, want string) error {
	v, err := w.jsonField(path)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%v", v) != want {
		return fmt.Errorf("%s = %v, want %s", path, v, want)
	}
	return nil
}

func (w *world) jsonFieldIsNumber(path string, want int) error {
	v, err := w.jsonField(path)
	if err != nil {
		return err
	}
	f, ok := v.(float64)
	if !ok || int(f) != want {
		return fmt.Errorf("%s = %v, want %d", path, v, want)
	}
	return nil
}

func (w *world) jsonFieldIsEmptyList(path string) error {
	v, err := w.jsonField(path)
	if err != nil {
		return err
	}
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%s is %T, want a list (null and [] differ to a consumer)", path, v)
	}
	if len(arr) != 0 {
		return fmt.Errorf("%s has %d entries, want none", path, len(arr))
	}
	return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	w := &world{}
	ctx.Before(func(c context.Context, _ *godog.Scenario) (context.Context, error) {
		w.reset()
		return c, nil
	})

	ctx.Given(`^the BOM fixture "([^"]*)"$`, w.theBOMFixture)
	ctx.Given(`^a BOM path that does not exist$`, w.aBOMPathThatDoesNotExist)
	ctx.When(`^I run "([^"]*)"$`, w.iRun)
	ctx.When(`^I run "([^"]*)" from "([^"]*)"$`, w.iRunFrom)

	ctx.Then(`^the command fails$`, w.theCommandFails)
	ctx.Then(`^stdout is empty$`, w.stdoutIsEmpty)
	ctx.Then(`^the output lists (\d+) components$`, w.theOutputListsComponents)
	ctx.Then(`^the output contains "([^"]*)"$`, w.theOutputContains)
	ctx.Then(`^the output reports coverage$`, w.theOutputReportsCoverage)
	ctx.Then(`^the output reports coverage of (\d+) out of (\d+)$`, w.theOutputReportsCoverageOf)
	ctx.Then(`^the output marks a back-reference$`, w.theOutputMarksABackReference)
	ctx.Then(`^the output marks a cycle$`, w.theOutputMarksACycle)
	ctx.Then(`^the output does not mark a cycle$`, w.theOutputDoesNotMarkACycle)
	ctx.Then(`^the output warns that not every component is shown$`, w.theOutputWarnsNotEveryComponentIsShown)
	ctx.Then(`^the output says the roots were derived$`, w.theOutputSaysRootsWereDerived)
	ctx.Then(`^the output explains that the BOM declares no dependency graph$`, w.theOutputExplainsNoDependencyGraph)
	ctx.Then(`^the output suggests listing by category$`, w.theOutputSuggestsListingByCategory)
	ctx.Then(`^the output explains that the component is unknown$`, w.theOutputExplainsUnknownComponent)
	ctx.Then(`^the output is valid JSON$`, w.theOutputIsValidJSON)
	ctx.Then(`^the JSON field "([^"]*)" is (true|false)$`, w.jsonFieldIsBool)
	ctx.Then(`^the JSON field "([^"]*)" is (\d+)$`, w.jsonFieldIsNumber)
	ctx.Then(`^the JSON field "([^"]*)" is an empty list$`, w.jsonFieldIsEmptyList)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{".."},
			Output:   colors.Colored(os.Stdout),
			TestingT: t,
			Strict:   true, // an undefined or pending step FAILS rather than passing
		},
	}
	if suite.Run() != 0 {
		t.Fatal("feature scenarios failed")
	}
}
