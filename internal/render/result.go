// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

// Package render turns a traversal into text or JSON.
//
// Both surfaces are produced from ONE result type. That is deliberate: ADR-0004
// makes coverage a correctness guarantee rather than a flag, and the cheapest way
// to keep a guarantee is to make it structurally impossible to omit. A renderer
// cannot forget to print coverage, because coverage is a field of the thing it is
// handed rather than something it must remember to fetch.
package render

import "github.com/jrjsmrtn/bomdive/internal/bom"

// Entry is one line of output.
type Entry struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Type     string `json:"type,omitempty"`
	PURL     string `json:"purl,omitempty"`
	Category string `json:"category,omitempty"`

	// Depth, Repeat and Cycle are meaningful for a tree and zero for a listing.
	Depth  int  `json:"depth,omitempty"`
	Repeat bool `json:"repeat,omitempty"`
	Cycle  bool `json:"cycle,omitempty"`

	// Children is how many components this one depends on. In a flat listing it is
	// what tells a reader whether descending is even possible — which is how a
	// sparse graph becomes visible rather than misleading.
	Children int `json:"children"`
}

// Label is the entry's display name, falling back to its ref. Mirrors
// bom.Node.Label so the two surfaces cannot diverge.
func (e Entry) Label() string {
	if e.Name != "" {
		return e.Name
	}
	if e.Ref != "" {
		return e.Ref
	}
	return "(unnamed)"
}

// Coverage mirrors bom.Coverage for serialisation, plus the derived percentage a
// human reader actually wants.
type Coverage struct {
	Components int      `json:"components"`
	InGraph    int      `json:"in_graph"`
	Percent    int      `json:"percent"`
	Edges      int      `json:"edges"`
	Complete   bool     `json:"complete"`
	Dangling   []string `json:"dangling,omitempty"`
}

// Result is everything a renderer needs. Every field that carries a caveat is
// mandatory rather than optional, so no surface can quietly drop one.
type Result struct {
	Command string `json:"command"`
	Source  string `json:"source"`

	// Kind and Identity say what the document claims to be, so a reader can tell
	// an empty result from a misread document.
	Kind     string `json:"kind"`
	Identity string `json:"identity"`

	// From is the ref that was listed or walked, empty for a whole-document listing.
	From string `json:"from,omitempty"`

	// SyntheticRoots is true when roots were derived rather than declared. A caller
	// MUST surface it: presenting an invented root as declared claims something the
	// document does not say.
	SyntheticRoots bool `json:"synthetic_roots"`

	// DeclaresNoGraph is the document asserting it has no relations, which is
	// different from a generator having computed none. HasDependencies records
	// whether the field was there at all.
	DeclaresNoGraph bool `json:"declares_no_graph"`
	HasDependencies bool `json:"has_dependencies"`

	// GraphState names which of the four cases this document is, so a consumer of
	// the JSON does not have to re-derive it from the two booleans above and get it
	// wrong the way this project's own surfaces did.
	GraphState string `json:"graph_state"`

	Coverage Coverage `json:"coverage"`
	Entries  []Entry  `json:"entries"`

	// Notes carry anything a reader must know to read the entries correctly.
	Notes []string `json:"notes,omitempty"`
}

func coverageOf(c bom.Coverage) Coverage {
	return Coverage{
		Components: c.Components, InGraph: c.InGraph, Percent: c.Percent(),
		Edges: c.Edges, Complete: c.Complete(), Dangling: c.Dangling,
	}
}

// stateName is the wire form of bom.GraphState. Spelled out rather than numeric so
// a JSON consumer reads a word instead of an integer whose meaning lives in Go.
func stateName(s bom.GraphState) string {
	switch s {
	case bom.GraphPartial:
		return "partial"
	case bom.GraphDeclaredEmpty:
		return "declared-empty"
	case bom.GraphUndeclared:
		return "undeclared"
	default:
		return "complete"
	}
}

func entryOf(g *bom.Graph, n bom.Node) Entry {
	return Entry{
		Ref: n.Ref, Name: n.Name, Version: n.Version, Type: n.Type,
		PURL: n.PURL, Category: n.Category, Children: len(g.Children(n.Ref)),
	}
}

// New builds the common part of a Result, so every command starts from the same
// mandatory caveats rather than assembling them by hand.
//
// The graph-state note is attached HERE, not per command. It used to be written
// out by the tree renderer alone, so `bomdive ls` and the column view printed a bare
// 0% with nothing to distinguish "the document says there are no relations" from
// "the document says nothing" from "relations exist and miss most components".
// Attaching it at construction is the same argument that put Coverage in this type:
// a caveat a renderer must remember to add is a caveat that will be forgotten.
func New(command, source string, g *bom.Graph) Result {
	id := g.Identity()
	cov := g.Coverage()
	r := Result{
		Command: command, Source: source,
		Kind: string(id.Kind), Identity: id.Describe(),
		DeclaresNoGraph: g.DeclaresNoGraph(),
		HasDependencies: g.HasDependenciesKey(),
		GraphState:      stateName(cov.State()),
		Coverage:        coverageOf(cov),
	}
	// An empty component list is explained INSTEAD of the graph state. With nothing
	// to inventory there is nothing for a graph to cover, so "0 of 0 because
	// `dependencies` is absent" answers a question nobody asked while leaving the
	// one they did ask — why is this empty — unanswered.
	if empty, isEmpty := g.Contents().ExplainEmpty(); isEmpty {
		r.Notes = append(r.Notes, empty)
	} else if cov.State() != bom.GraphComplete {
		r.Notes = append(r.Notes, cov.Explain())
	}
	return r
}
