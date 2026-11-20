package docparse

import (
	"fmt"
	"regexp"
	"strings"
)

// Document is a parsed guide. It holds only the fields the MCP server serves;
// format specific extras (like the DocBook provenance) hang off the Extras
// slot, which a parser sets and which is not serialized on its own.
type Document struct {
	ID       string    `json:"id" yaml:"id"`
	Title    string    `json:"title" yaml:"title"`
	Summary  string    `json:"summary" yaml:"summary"`
	Distro   Distro    `json:"distro" yaml:"distro"`
	Product  Product   `json:"product" yaml:"product"`
	Chapters []Chapter `json:"chapters" yaml:"chapters"`
	License  string    `json:"license,omitempty" yaml:"license,omitempty"`

	// Extras carries format specific document data, set by the parser that
	// produced the document (see the docbook package). It is not part of the
	// MCP surface and is not serialized by the core types.
	Extras DocumentExtras `json:"-" yaml:"-"`
}

// Distro is the distribution a document applies to, e.g. "SUSE Linux
// Enterprise Server" and its versions (16.0, 16.1, ...).
type Distro struct {
	Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
	Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`
}

// Product is a specific product (e.g. "SLES for SAP Applications") a document
// applies to. It is distinct from the Distro: both can carry a version.
type Product struct {
	Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
	Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`
}

// Chapter is a top level section of a document.
type Chapter struct {
	ID       string    `json:"id" yaml:"id"`
	Title    string    `json:"chapter_title" yaml:"chapter_title"`
	Summary  string    `json:"summary,omitempty" yaml:"summary,omitempty"`   // lead paragraphs
	File     string    `json:"file,omitempty" yaml:"file,omitempty"`         // file the chapter was written in
	Commands []string  `json:"commands,omitempty" yaml:"commands,omitempty"` // shell commands shown in the chapter
	Sections []Section `json:"sections,omitempty" yaml:"sections,omitempty"` // headings below the chapter
	Xrefs    []string  `json:"xrefs,omitempty" yaml:"xrefs,omitempty"`       // linkends of the cross references, see Corpus.XrefTargets

	LinesKey string `json:"-" yaml:"-"` // key of the source lines, see Corpus.AddLines
	Start    uint   `json:"-" yaml:"-"` // first line of the chapter in the source lines, 1-based
	End      uint   `json:"-" yaml:"-"` // last line of the chapter in the source lines
	Prose    string `json:"-" yaml:"-"` // text outside of code blocks, used for glossary matching and search

	// Extra carries format specific chapter data, set by the parser that
	// produced the chapter. It is not part of the MCP surface and is not
	// serialized by the core types.
	Extra ChapterExtras `json:"-" yaml:"-"`
}

// Section is a heading below the chapter heading. Level 1 is a direct
// subsection of the chapter. File is set for sections taken from another
// topic file (nested modules). Number is the section number as published,
// e.g. "19.6.9", empty for unnumbered sections.
type Section struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Number string `json:"number,omitempty" yaml:"number,omitempty"`
	Title  string `json:"title" yaml:"title"`
	Level  int    `json:"level" yaml:"level"`
	File   string `json:"file,omitempty" yaml:"file,omitempty"`
}

// XrefTarget is a cross reference of a chapter resolved against the whole
// corpus: where its linkend points to (a chapter or a section within one), or
// a report that it does not resolve anywhere. Resolved is false when the
// linkend is not a navigable target in the corpus, in which case only Linkend
// and Text are set.
type XrefTarget struct {
	Linkend   string `json:"linkend"`
	Text      string `json:"text,omitempty"`
	Resolved  bool   `json:"resolved"`
	DocID     string `json:"doc_id,omitempty"`
	ChapterID string `json:"chapter_id,omitempty"`
	SectionID string `json:"section_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

// GlossaryEntry is a term and its definition.
type GlossaryEntry struct {
	Term   string `json:"term" yaml:"term"`
	WhatIs string `json:"whatis" yaml:"whatis"`

	match   []*regexp.Regexp // one per alias, see glossaryMatchers
	needles []string         // one per alias, see glossaryNeedles
}

var reNonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a title into an id: "1.2 What's new?" -> "1-2-what-s-new".
func Slug(s string) string {
	s = strings.Trim(reNonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if s == "" {
		return "chapter"
	}
	return s
}

// UniqueID returns id, or id with a numeric suffix if it was handed out before.
func UniqueID(seen map[string]int, id string) string {
	seen[id]++
	if n := seen[id]; n > 1 {
		return fmt.Sprintf("%s-%d", id, n)
	}
	return id
}

// DocumentExtras is set by a parser on Document.Extras to carry format
// specific document data (like the DocBook provenance). It is not serialized
// by the core types; the dump layer reads it back through the concrete type.
type DocumentExtras interface {
	// DocumentExtra marks a value as implementing DocumentExtras.
	DocumentExtra()
}

// ChapterExtras is set by a parser on Chapter.Extra to carry format specific
// chapter data. It is not serialized by the core types; the dump layer reads
// it back through the concrete type.
type ChapterExtras interface {
	// ChapterExtra marks a value as implementing ChapterExtras.
	ChapterExtra()
}
