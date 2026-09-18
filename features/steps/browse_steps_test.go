// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/columns"
)

// browseWorld drives the column model, which is the part of `browse` that has a contract.
// cli.Run is not usable here: browse refuses without a terminal, deliberately.
type browseWorld struct {
	graph *bom.Graph
	model *columns.Model
}

func (w *browseWorld) reset() { *w = browseWorld{} }

func (w *browseWorld) theDocument(name string) error {
	g, err := bom.Load(filepath.Join("..", "..", "testdata", name+".cdx.json"))
	if err != nil {
		return err
	}
	w.graph = g
	return nil
}

func (w *browseWorld) iBrowseIt() error {
	if w.graph == nil {
		return fmt.Errorf("no document loaded")
	}
	w.model = columns.New(w.graph)
	return nil
}

func (w *browseWorld) entryColumnTitled(want string) error {
	if got := w.model.Columns()[0].Title; got != want {
		return fmt.Errorf("entry column is titled %q, want %q", got, want)
	}
	return nil
}

func (w *browseWorld) entryColumnLists(n int) error {
	if got := len(w.model.Columns()[0].Entries); got != n {
		return fmt.Errorf("entry column lists %d rows, want %d", got, n)
	}
	return nil
}

func (w *browseWorld) viewSaysShowing(want string) error {
	if got := w.model.Showing(); got != want {
		return fmt.Errorf("the view says it is showing %q, want %q", got, want)
	}
	return nil
}

func (w *browseWorld) iDescend() error {
	if !w.model.Right() {
		return fmt.Errorf("could not descend from %q", w.model.Active().Title)
	}
	return nil
}

func (w *browseWorld) iGoBack() error {
	if !w.model.Left() {
		return fmt.Errorf("could not go back")
	}
	return nil
}

func (w *browseWorld) thereAreColumns(n int) error {
	if got := len(w.model.Columns()); got != n {
		return fmt.Errorf("there are %d columns, want %d", got, n)
	}
	return nil
}

// The path is the answer to "what pulled this in", kept on screen rather than traced back
// through indentation. It carries labels, not purls: a component with no name falls back to its
// bom-ref, which is why Label() decides and not the raw field.
func (w *browseWorld) pathReads(want string) error {
	if got := strings.Join(w.model.Path(), " > "); got != want {
		return fmt.Errorf("the path reads %q, want %q", got, want)
	}
	return nil
}

func (w *browseWorld) iFlipTheDirection() error {
	w.model.ToggleDirection()
	return nil
}

func (w *browseWorld) coverageBelow(pct int) error {
	if got := w.graph.Coverage().Percent(); got >= pct {
		return fmt.Errorf("coverage is %d%%, want below %d%%", got, pct)
	}
	return nil
}

// The correctness obligation: a partial graph must say so, on every surface. ls and tree are
// covered by their own features; this is the same promise in the column view.
func (w *browseWorld) viewExplainsCoverage() error {
	if got := strings.TrimSpace(w.graph.Coverage().Explain()); got == "" {
		return fmt.Errorf("the view offers no explanation of its coverage")
	}
	return nil
}

func initializeBrowseSteps(ctx *godog.ScenarioContext) {
	w := &browseWorld{}
	ctx.Before(func(c context.Context, _ *godog.Scenario) (context.Context, error) {
		w.reset()
		return c, nil
	})

	// Deliberately phrased differently from ls/tree's "BOM fixture": browse opens documents
	// that are not all bills of materials, and two identical regexes are an ambiguous step.
	ctx.Given(`^the document "([^"]*)"$`, w.theDocument)
	ctx.When(`^I browse it$`, w.iBrowseIt)
	ctx.When(`^I descend$`, w.iDescend)
	ctx.When(`^I go back$`, w.iGoBack)
	ctx.When(`^I flip the direction$`, w.iFlipTheDirection)

	ctx.Then(`^the entry column is titled "([^"]*)"$`, w.entryColumnTitled)
	ctx.Then(`^the entry column lists (\d+) row$`, w.entryColumnLists)
	ctx.Then(`^the view says it is showing "([^"]*)"$`, w.viewSaysShowing)
	ctx.Then(`^there are (\d+) columns$`, w.thereAreColumns)
	ctx.Then(`^the path reads "([^"]*)"$`, w.pathReads)
	ctx.Then(`^the view reports coverage below (\d+) percent$`, w.coverageBelow)
	ctx.Then(`^the view explains the coverage$`, w.viewExplainsCoverage)
}
