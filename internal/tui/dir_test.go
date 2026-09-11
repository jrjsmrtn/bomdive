package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/lsxbom/internal/bom"
)

// openDir builds a directory holding two linked documents, a CycloneDX file that fails
// to load and a README, and opens it the way `browse DIR` does (ADR-0009 [D]).
func openDir(t *testing.T) *ui {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"link-sbom", "link-vex"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", n+".cdx.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, n+".cdx.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := `{"bomFormat":"CycloneDX","specVersion":"1.6","components":{"x":1}}`
	if err := os.WriteFile(filepath.Join(dir, "broken.cdx.json"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := bom.LoadArgs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(s.Doc(0), s.Doc(0).Path())
	u.width = 120
	return u
}

// On a file that failed to load, the header, status bar and ? describe THAT file —
// not the first document, which is what Graph() falls back to.
func TestAFileThatFailedToLoadIsDescribedAsSuch(t *testing.T) {
	u := openDir(t)
	if h := u.headerText(); !strings.Contains(h, "not loaded:") || strings.Contains(h, "SBOM") {
		t.Errorf("header = %q, want the failure and not the SBOM's identity", h)
	}
	if s := u.statusText(); !strings.Contains(s, "not loaded") || strings.Contains(s, "coverage") {
		t.Errorf("status = %q, want not loaded and no coverage", s)
	}
	if e := u.explainText(); !strings.Contains(e, "could not be read") || !strings.Contains(e, "broken.cdx.json") {
		t.Errorf("? = %q, want the failure explained", e)
	}
	u.model.Down()
	if h := u.headerText(); strings.Contains(h, "not loaded") || !strings.Contains(h, "SBOM") {
		t.Errorf("header on the SBOM = %q", h)
	}
}

// A skipped file is not a row, so ? is where it is named, with the reason.
func TestQuestionMarkNamesTheSkippedFiles(t *testing.T) {
	u := openDir(t)
	u.model.Down()
	ax := u.axisText()
	for _, want := range []string{"directly inside", "1 file(s) not loaded", "1 file(s) skipped", "README.md — "} {
		if !strings.Contains(ax, want) {
			t.Errorf("? does not say %q: %q", want, ax)
		}
	}
}
