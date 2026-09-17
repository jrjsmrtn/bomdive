// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/bomdive/internal/bom"
)

// openDir opens a directory holding two linked documents, a CycloneDX file that fails
// to load, a README and a sub-directory (ADR-0009 [D]).
func openDir(t *testing.T) (*Model, string) {
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
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := bom.LoadArgs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	return New(s.Doc(0)), dir
}

// The files column says where the set came from, and counts what it left out.
func TestTheFilesColumnSaysWhereTheSetCameFrom(t *testing.T) {
	m, dir := openDir(t)
	want := "2 documents from " + dir + ", 1 not loaded, 2 skipped"
	if got := m.Columns()[0].Title; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got := strings.Join(rowNames(m.Columns()[0].Entries), " | "); got !=
		"broken.cdx.json — not loaded | link-sbom.cdx.json — SBOM | link-vex.cdx.json — VEX" {
		t.Errorf("rows = %q", got)
	}
}

// Named files keep their plain title: nothing about their origin needs saying.
func TestNamedFilesKeepThePlainTitle(t *testing.T) {
	m := openSet(t, "link-sbom", "link-vex")
	if got := m.Columns()[0].Title; got != "files" {
		t.Errorf("title = %q, want files", got)
	}
}

// A CycloneDX file that failed to load is a row that cannot be opened and says why —
// and selecting it describes no other document.
func TestAFileThatFailedToLoadIsARowThatSaysWhy(t *testing.T) {
	m, _ := openDir(t)
	sel, _ := m.Active().Selected()
	if m.Descendable(sel) || m.Right() || len(m.childrenOf(sel)) != 0 {
		t.Error("the failed file can be descended into")
	}
	mem, ok := m.Unloaded()
	if !ok || filepath.Base(mem.Path) != "broken.cdx.json" {
		t.Fatalf("Unloaded = %+v, %v", mem, ok)
	}
	kv := m.Detail()
	if got := detailValue(kv, "not loaded"); got == "" || got != mem.Reason() {
		t.Errorf("detail says %q, want the reason %q", got, mem.Reason())
	}
	_ = m.Graph() // must not panic on a row with no document behind it
	m.Down()
	if _, ok := m.Unloaded(); ok {
		t.Error("a loaded file reads as not loaded")
	}
	if !m.Right() {
		t.Error("could not open the loaded file below the failed one")
	}
}

// A directory holding one document and one CycloneDX file that failed still shows the
// files column: the failure is a row, and a set of one document plus a failure is not a
// set of one — hiding the column would hide the failure.
func TestOneDocumentAndOneFailureStillShowTheFiles(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "link-sbom.cdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	broken := `{"bomFormat":"CycloneDX","specVersion":"1.6","components":{"x":1}}`
	for name, body := range map[string][]byte{"a.cdx.json": raw, "b.cdx.json": []byte(broken)} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := bom.LoadArgs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	m := New(s.Doc(0))
	if m.Axis() != AxisFiles || len(m.Columns()[0].Entries) != 2 {
		t.Fatalf("axis %v with %d rows, want the files column with 2", m.Axis(), len(m.Columns()[0].Entries))
	}
	if got, want := m.Columns()[0].Title, "1 document from "+dir+", 1 not loaded"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}
