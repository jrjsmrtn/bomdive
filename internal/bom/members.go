// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

package bom

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// ErrNotCycloneDX marks a file that is not a CycloneDX document at all, as opposed to
// one that claims to be and fails to load. In a named directory the first kind is
// skipped and named by `?`; the second is shown as a row, because hiding it would
// render a partial set as complete (ADR-0009 [D]).
var ErrNotCycloneDX = errors.New("not a CycloneDX document")

// notCycloneDX wraps a load error as ErrNotCycloneDX while keeping its message, which
// says what the file is instead ("not JSON or XML", "bomFormat=\"\"").
type notCycloneDX struct{ error }

func (notCycloneDX) Is(target error) bool { return target == ErrNotCycloneDX }
func (e notCycloneDX) Unwrap() error      { return e.error }

// claimsCycloneDX reports whether a document that failed to decode nevertheless says
// it is CycloneDX: a JSON `bomFormat` of "CycloneDX", or an XML root in the CycloneDX
// namespace.
//
// The JSON check reads TOKENS rather than unmarshalling, so a truncated document that
// declared its format before breaking still counts as CycloneDX — a failed load, shown
// as a row — rather than as a stranger to be skipped. encoding/xml expands no DTD
// entities, so reading an untrusted XML root is safe.
func claimsCycloneDX(raw []byte, format cdx.BOMFileFormat) bool {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if format == cdx.BOMFileFormatXML {
		d := xml.NewDecoder(bytes.NewReader(raw))
		for {
			tok, err := d.Token()
			if err != nil {
				return false
			}
			if el, ok := tok.(xml.StartElement); ok {
				return strings.Contains(el.Name.Space, "cyclonedx.org/schema/bom")
			}
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
		return false
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return false
		}
		if key == "bomFormat" {
			var v string
			return d.Decode(&v) == nil && v == "CycloneDX"
		}
		var skip json.RawMessage
		if d.Decode(&skip) != nil {
			return false
		}
	}
	return false
}

// Member is one row of the files column: a loaded document (Doc is its index), or a
// CycloneDX file that failed to load (Doc is -1 and Err says why). Named is true for a
// file named on the command line rather than found in a named directory.
type Member struct {
	Path  string
	Doc   int
	Err   error
	Named bool
}

// Reason is why a member failed to load, without the path the error repeats.
func (m Member) Reason() string {
	if m.Err == nil {
		return ""
	}
	return strings.TrimPrefix(m.Err.Error(), m.Path+": ")
}

// Skipped is a file in a named directory that was not loaded because it is not a
// CycloneDX document, or is not a file. It is counted and named, never a row, and
// never dropped silently.
type Skipped struct {
	Path   string
	Reason string
}

// LoadArgs loads the documents named on the command line into one session (ADR-0009
// [D]). A file is loaded as named, and a file that fails stops everything: the user
// asked for it. A directory stands for the documents directly inside it, sorted by
// name, at its place in the arguments: a file that is not CycloneDX is skipped and
// counted, a CycloneDX file that fails to load becomes a row that cannot be opened,
// and a sub-directory is skipped, because a directory is read one level deep. A file
// reached twice is loaded once.
func LoadArgs(args []string) (*Set, error) {
	var (
		gs      []*Graph
		members []Member
		skipped []Skipped
		dirs    []string
		// seen maps a file to whether it LOADED. A file a directory skipped or failed to
		// load is loaded again if it is also named, so it fails hard: without that, naming
		// a broken file after its own directory passed silently — the directory had
		// reached it first, and "loaded once" swallowed the explicit request.
		seen = map[string]bool{}
	)
	add := func(p string, named bool) error {
		key := filepath.Clean(p)
		if abs, err := filepath.Abs(p); err == nil {
			key = abs
		}
		loaded, reached := seen[key]
		if loaded || (reached && !named) {
			return nil
		}
		g, err := Load(p)
		seen[key] = err == nil
		switch {
		case err == nil:
			members = append(members, Member{Path: p, Doc: len(gs), Named: named})
			gs = append(gs, g)
		case named:
			return err
		case errors.Is(err, ErrNotCycloneDX):
			skipped = append(skipped, Skipped{Path: p, Reason: strings.TrimPrefix(err.Error(), p+": ")})
		default:
			members = append(members, Member{Path: p, Doc: -1, Err: err})
		}
		return nil
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if err := add(a, true); err != nil {
				return nil, err
			}
			continue
		}
		dirs = append(dirs, a)
		entries, err := os.ReadDir(a)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			p := filepath.Join(a, e.Name())
			info, err := os.Stat(p) // follows a symlink to what it names
			switch {
			case err != nil:
				skipped = append(skipped, Skipped{Path: p, Reason: err.Error()})
			case info.IsDir():
				skipped = append(skipped, Skipped{Path: p, Reason: "a directory: a named directory is read one level deep"})
			case !info.Mode().IsRegular():
				skipped = append(skipped, Skipped{Path: p, Reason: "not a regular file"})
			default:
				_ = add(p, false) // only a NAMED file returns an error
			}
		}
	}
	if len(gs) == 0 {
		failed, first := 0, ""
		for _, m := range members {
			failed++
			if first == "" {
				first = m.Err.Error()
			}
		}
		msg := fmt.Sprintf("no CycloneDX document loaded from %s: %d file(s) not CycloneDX, %d failed to load",
			strings.Join(args, ", "), len(skipped), failed)
		if first != "" {
			msg += "; the first failure: " + first
		}
		return nil, errors.New(msg)
	}
	s := NewSet(gs...)
	s.members, s.skipped, s.dirs = members, skipped, dirs
	return s, nil
}

// Members are the rows of the files column, in the order reached.
func (s *Set) Members() []Member { return s.members }

// Skipped are the files in named directories that were not CycloneDX documents.
func (s *Set) Skipped() []Skipped { return s.skipped }

// Directories are the directories named on the command line, in the order given.
func (s *Set) Directories() []string { return s.dirs }
