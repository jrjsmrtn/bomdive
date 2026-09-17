// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// browse accepts a directory (ADR-0009 [D]). Without a terminal it stops at the
// terminal check — which it reaches only if the directory loaded.
func TestBrowseAcceptsADirectory(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(fx("link-sbom"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.cdx.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run("browse", dir)
	if code == 0 || !strings.Contains(stderr, "interactive terminal") {
		t.Errorf("browse DIR: code %d, stderr %q — want it to load and stop at the terminal check", code, stderr)
	}

	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "README.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = run("browse", empty)
	if code == 0 || !strings.Contains(stderr, "no CycloneDX document loaded") {
		t.Errorf("browse on a directory with no document: code %d, stderr %q", code, stderr)
	}
}
