// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package bom

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Whether a failure is skipped or shown decides whether a directory can hide a document
// (ADR-0009 [D]). A file that says it is CycloneDX and fails is a FAILED load, shown as
// a row; anything else is not CycloneDX, and is skipped.
func TestALoadErrorSaysWhetherTheFileIsCycloneDX(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		name, body string
		notCDX     bool
	}{
		{"text.txt", "not json at all", true},
		{"empty.json", "", true},
		{"spdx.json", `{"spdxVersion":"SPDX-2.3","name":"x"}`, true},
		{"array.json", `[1, 2]`, true},
		{"pom.xml", `<project xmlns="http://maven.apache.org/POM/4.0.0"/>`, true},
		{"bad-type.cdx.json", `{"bomFormat":"CycloneDX","specVersion":"1.6","components":{"x":1}}`, false},
		// Truncated AFTER declaring its format, with the declaration not first.
		{"truncated.cdx.json", `{"specVersion":"1.6","bomFormat":"CycloneDX","components":[`, false},
		{"no-spec.cdx.json", `{"bomFormat":"CycloneDX"}`, false},
		{"truncated.cdx.xml", `<bom xmlns="http://cyclonedx.org/schema/bom/1.6"><components>`, false},
	} {
		p := filepath.Join(dir, c.name)
		if err := os.WriteFile(p, []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(p)
		if err == nil {
			t.Errorf("%s loaded", c.name)
			continue
		}
		if got := errors.Is(err, ErrNotCycloneDX); got != c.notCDX {
			t.Errorf("%s: not CycloneDX = %v, want %v (%v)", c.name, got, c.notCDX, err)
		}
	}
}

// writeDir builds a directory shaped like a real one: two linked documents, a README,
// a CycloneDX file that fails to load, and a sub-directory.
func writeDir(t *testing.T) string {
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
	files := map[string]string{
		"README.md":       "# notes\n",
		"broken.cdx.json": `{"bomFormat":"CycloneDX","specVersion":"1.6","components":{"x":1}}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func memberRows(s *Set) string {
	var rows []string
	for _, m := range s.Members() {
		state := "loaded"
		if m.Err != nil {
			state = "failed"
		}
		rows = append(rows, filepath.Base(m.Path)+" "+state)
	}
	return strings.Join(rows, ", ")
}

// A directory stands for the documents directly inside it, sorted by name: the failed
// CycloneDX file is a row, the README and the sub-directory are skipped and named.
func TestADirectoryStandsForTheDocumentsInsideIt(t *testing.T) {
	dir := writeDir(t)
	s, err := LoadArgs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := memberRows(s), "broken.cdx.json failed, link-sbom.cdx.json loaded, link-vex.cdx.json loaded"; got != want {
		t.Errorf("members = %q, want %q", got, want)
	}
	var skipped []string
	for _, k := range s.Skipped() {
		skipped = append(skipped, filepath.Base(k.Path))
	}
	if got := strings.Join(skipped, " "); got != "README.md nested" {
		t.Errorf("skipped = %q, want README.md nested", got)
	}
	// The reason is what ? shows: a sub-directory is left out because a directory is
	// read one level deep, which is a different thing from "not a regular file".
	if got := s.Skipped()[1].Reason; !strings.Contains(got, "one level deep") {
		t.Errorf("the sub-directory's reason = %q, want it to say one level deep", got)
	}
	if s.Len() != 2 || strings.Join(s.Directories(), " ") != dir {
		t.Errorf("%d documents from %v, want 2 from %s", s.Len(), s.Directories(), dir)
	}
	if m := s.Members()[0]; m.Named || !strings.Contains(m.Reason(), "json") || strings.HasPrefix(m.Reason(), m.Path) {
		t.Errorf("failed member = %+v, reason %q: want unnamed, with a reason that does not repeat the path", m, m.Reason())
	}
}

// Links resolve among a directory's documents exactly as among the same files named.
func TestLinksResolveInADirectoryAsAmongNamedFiles(t *testing.T) {
	fromDir, err := LoadArgs([]string{writeDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	named := loadSet(t, "link-sbom", "link-vex")
	a, b := fromDir.Doc(1), named.Doc(1)
	for i, v := range a.Vulnerabilities() {
		for j, ref := range v.Affects {
			_, got := a.Resolve(ref.Ref)
			_, want := b.Resolve(b.Vulnerabilities()[i].Affects[j].Ref)
			if got != want {
				t.Errorf("%s -> %s: %v from the directory, %v named", v.ID, ref.Ref, got, want)
			}
		}
	}
}

// A file named on its own must load: the user asked for it — whichever order it is
// named in relative to a directory that also holds it.
func TestANamedFileStillFailsHard(t *testing.T) {
	dir := writeDir(t)
	for _, name := range []string{"broken.cdx.json", "README.md"} {
		file := filepath.Join(dir, name)
		if _, err := LoadArgs([]string{dir, file}); err == nil {
			t.Errorf("naming %s after its directory loaded", name)
		}
		if _, err := LoadArgs([]string{file, dir}); err == nil {
			t.Errorf("naming %s before its directory loaded", name)
		}
	}
	if _, err := LoadArgs([]string{filepath.Join(dir, "no-such.cdx.json")}); err == nil {
		t.Error("a missing argument loaded")
	}
}

// A file reached twice is loaded once, and a file named beside the directory is marked
// as named.
func TestAFileReachedTwiceIsLoadedOnce(t *testing.T) {
	dir := writeDir(t)
	v2 := filepath.Join("..", "..", "testdata", "link-sbom-v2.cdx.json")
	s, err := LoadArgs([]string{dir, filepath.Join(dir, "link-vex.cdx.json"), v2})
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != 3 || len(s.Members()) != 4 {
		t.Errorf("%d documents in %d rows, want 3 in 4: %s", s.Len(), len(s.Members()), memberRows(s))
	}
	last := s.Members()[3]
	if !last.Named || s.Members()[1].Named {
		t.Errorf("named flags: last %v, a directory member %v — want true, false", last.Named, s.Members()[1].Named)
	}
}

// A directory with nothing that loads is an error that says what was there.
func TestADirectoryWithNothingLoadableIsAnError(t *testing.T) {
	dir := writeDir(t)
	for _, n := range []string{"link-sbom", "link-vex"} {
		if err := os.Remove(filepath.Join(dir, n+".cdx.json")); err != nil {
			t.Fatal(err)
		}
	}
	_, err := LoadArgs([]string{dir})
	if err == nil || !strings.Contains(err.Error(), "2 file(s) not CycloneDX, 1 failed to load") {
		t.Errorf("err = %v, want one counting 2 skipped and 1 failed", err)
	}
}
