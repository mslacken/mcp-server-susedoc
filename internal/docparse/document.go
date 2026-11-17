package docparse

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Document is a parsed guide.
type Document struct {
	ID       string    `json:"id" yaml:"id"`
	Title    string    `json:"title" yaml:"title"`
	Summary  string    `json:"summary" yaml:"summary"`
	Distro   Distro    `json:"distro" yaml:"distro"`
	Product  Product   `json:"product" yaml:"product"`
	Chapters []Chapter `json:"chapters" yaml:"chapters"`
	License  string    `json:"license,omitempty" yaml:"license,omitempty"`

	// DocBook assemblies only; not exposed via MCP, hence unexported. They are
	// still serialized by the CLI dump via MarshalJSON/MarshalYAML.
	source      *DocSource
	profile     *Profile
	subtitle    string
	meta        *DocMeta
	revisions   []Revision
	bugtracker  *Bugtracker
	translation string // "yes" or "no"
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
	ID         string   `json:"id" yaml:"id"`
	Title      string   `json:"chapter_title" yaml:"chapter_title"`
	Summary    string   `json:"summary,omitempty" yaml:"summary,omitempty"`         // lead paragraphs
	Start      uint     `json:"start" yaml:"start"`                                 // first line, counted in the original source file
	End        uint     `json:"end" yaml:"end"`                                     // last line, counted in the original source file
	FilePath   string   `json:"file_path" yaml:"file_path"`                         // file the chapter was parsed from (preferred)
	SourcePath string   `json:"source_path,omitempty" yaml:"source_path,omitempty"` // original file before combining
	Commands   []string `json:"commands,omitempty" yaml:"commands,omitempty"`

	// DocBook assemblies only; not exposed via MCP, hence unexported. They are
	// still serialized by the CLI dump via Chapter.MarshalJSON/MarshalYAML.
	number        string // as in the published document: "19" (chapter), "A" (appendix), "3" (section of an article)
	topicType     string // task, concept, reference, glue or container
	renderAs      string // section, appendix, ...
	maintainer    string
	legacySources []string // "refers to" URLs of the original documentation
	sections      []Section
	procedures    []Procedure
	admonitions   []Admonition
	xrefs         []Xref
	links         []Link
	images        []Image
	includes      []string // XIncluded snippet files

	// filled in by the parsers, not serialized
	LinesKey string `json:"-" yaml:"-"` // key of the source lines, see Corpus.AddLines
	Prose    string `json:"-" yaml:"-"` // text outside of code blocks, used for glossary matching and search
}

// GlossaryEntry is a term and its definition.
type GlossaryEntry struct {
	Term   string `json:"term" yaml:"term"`
	WhatIs string `json:"whatis" yaml:"whatis"`

	match   []*regexp.Regexp // one per alias, see glossaryMatchers
	needles []string         // one per alias, see glossaryNeedles
}

// DocSource records where a DocBook document came from.
type DocSource struct {
	DC        string `json:"dc" yaml:"dc"`                                   // DC file, relative to the corpus root
	Assembly  string `json:"assembly" yaml:"assembly"`                       // assembly file, relative to the corpus root
	Structure string `json:"structure,omitempty" yaml:"structure,omitempty"` // xml:id of the selected <structure>
}

// Profile is the profiling a DC file applies (PROFOS, PROFARCH, PROFCONDITION).
type Profile struct {
	OS        []string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch      []string `json:"arch,omitempty" yaml:"arch,omitempty"`
	Condition []string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// DocMeta is the <meta> information of an assembly.
type DocMeta struct {
	Maintainer        string   `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	Series            string   `json:"series,omitempty" yaml:"series,omitempty"`
	Tasks             []string `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Categories        []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Architectures     []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`
	Updated           string   `json:"updated,omitempty" yaml:"updated,omitempty"`
	Title             string   `json:"title,omitempty" yaml:"title,omitempty"` // title for search engines
	Description       string   `json:"description,omitempty" yaml:"description,omitempty"`
	SocialDescription string   `json:"social_description,omitempty" yaml:"social_description,omitempty"`
}

// Revision is an entry of the revision history.
type Revision struct {
	Date        string `json:"date,omitempty" yaml:"date,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Bugtracker is where documentation bugs are reported (docmanager metadata).
type Bugtracker struct {
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	Component string `json:"component,omitempty" yaml:"component,omitempty"`
	Product   string `json:"product,omitempty" yaml:"product,omitempty"`
	Assignee  string `json:"assignee,omitempty" yaml:"assignee,omitempty"`
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

// Procedure is a numbered list of steps. Number is the label of a titled
// procedure, e.g. "19.1" for "Procedure 19.1".
type Procedure struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Number string `json:"number,omitempty" yaml:"number,omitempty"`
	Title  string `json:"title,omitempty" yaml:"title,omitempty"`
	Steps  int    `json:"steps" yaml:"steps"`
}

// Admonition is a note, tip, important, warning or caution box.
type Admonition struct {
	Type  string `json:"type" yaml:"type"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// Xref is a cross reference; Resolved is false if the target is not part of
// the document.
type Xref struct {
	Linkend  string `json:"linkend" yaml:"linkend"`
	Resolved bool   `json:"resolved" yaml:"resolved"`
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

// Link is an external link.
type Link struct {
	URL  string `json:"url" yaml:"url"`
	Text string `json:"text,omitempty" yaml:"text,omitempty"`
}

// Image is an image shown in the chapter; File is relative to the corpus root.
type Image struct {
	File string `json:"file" yaml:"file"`
	Alt  string `json:"alt,omitempty" yaml:"alt,omitempty"`
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

// The DocBook-only fields of a Document are not exposed via MCP, so they are
// unexported. The accessors below let the parsers (in other packages) set them
// and the CLI dump marshal them to JSON and YAML with the same keys as before.

// Source returns where a DocBook document came from, or nil.
func (d *Document) Source() *DocSource { return d.source }

// Profile returns the profiling a DC file applies, or nil.
func (d *Document) Profile() *Profile { return d.profile }

// Subtitle returns the document subtitle, if any.
func (d *Document) Subtitle() string { return d.subtitle }

// Meta returns the <meta> information of an assembly, or nil.
func (d *Document) Meta() *DocMeta { return d.meta }

// Revisions returns the entries of the revision history.
func (d *Document) Revisions() []Revision { return d.revisions }

// Bugtracker returns where documentation bugs are reported, or nil.
func (d *Document) Bugtracker() *Bugtracker { return d.bugtracker }

// Translation reports whether the document is a translation; "yes" or "no".
func (d *Document) Translation() string { return d.translation }

// SetSource sets where a DocBook document came from.
func (d *Document) SetSource(v *DocSource) { d.source = v }

// SetProfile sets the profiling a DC file applies.
func (d *Document) SetProfile(v *Profile) { d.profile = v }

// SetSubtitle sets the document subtitle.
func (d *Document) SetSubtitle(v string) { d.subtitle = v }

// SetMeta sets the <meta> information of an assembly; pass nil to clear it.
func (d *Document) SetMeta(v *DocMeta) { d.meta = v }

// SetRevisions replaces the revision history.
func (d *Document) SetRevisions(v []Revision) { d.revisions = v }

// AddRevision appends an entry to the revision history.
func (d *Document) AddRevision(v Revision) { d.revisions = append(d.revisions, v) }

// SetBugtracker sets where documentation bugs are reported.
func (d *Document) SetBugtracker(v *Bugtracker) { d.bugtracker = v }

// SetTranslation sets whether the document is a translation.
func (d *Document) SetTranslation(v string) { d.translation = v }

// docMeta is the serialization form of a Document; the DocBook-only fields are
// included so the CLI dump keeps the same shape as before unexporting them.
type docMeta struct {
	ID          string      `json:"id" yaml:"id"`
	Title       string      `json:"title" yaml:"title"`
	Summary     string      `json:"summary" yaml:"summary"`
	Distro      Distro      `json:"distro" yaml:"distro"`
	Product     Product     `json:"product" yaml:"product"`
	Chapters    []Chapter   `json:"chapters" yaml:"chapters"`
	License     string      `json:"license,omitempty" yaml:"license,omitempty"`
	Source      *DocSource  `json:"source,omitempty" yaml:"source,omitempty"`
	Profile     *Profile    `json:"profile,omitempty" yaml:"profile,omitempty"`
	Subtitle    string      `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`
	Meta        *DocMeta    `json:"meta,omitempty" yaml:"meta,omitempty"`
	Revisions   []Revision  `json:"revisions,omitempty" yaml:"revisions,omitempty"`
	Bugtracker  *Bugtracker `json:"bugtracker,omitempty" yaml:"bugtracker,omitempty"`
	Translation string      `json:"translation,omitempty" yaml:"translation,omitempty"`
}

func (d *Document) docMeta() docMeta {
	return docMeta{ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
		Chapters: d.Chapters, License: d.License, Source: d.source, Profile: d.profile, Subtitle: d.subtitle,
		Meta: d.meta, Revisions: d.revisions, Bugtracker: d.bugtracker, Translation: d.translation}
}

// MarshalJSON implements json.Marshaler so the DocBook-only fields are still
// serialized even though they are unexported. The value receiver lets the
// method apply to the Document values stored in Corpus.Documents.
func (d Document) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.docMeta())
}

// MarshalYAML implements yaml.Marshaler (gopkg.in/yaml.v3) so the DocBook-only
// fields are still serialized even though they are unexported.
func (d Document) MarshalYAML() (interface{}, error) {
	return d.docMeta(), nil
}

// The DocBook-only fields of a Chapter are not exposed via MCP, so they are
// unexported. The accessors below let the docbook parser (in another package)
// set them, and the CLI dump marshal them to JSON and YAML with the same keys
// as before unexporting them.

// Number returns the chapter number as published, e.g. "19" or "A".
func (ch *Chapter) Number() string { return ch.number }

// TopicType returns the topic type: task, concept, reference, glue or container.
func (ch *Chapter) TopicType() string { return ch.topicType }

// RenderAs returns how the chapter is rendered: section, appendix, ...
func (ch *Chapter) RenderAs() string { return ch.renderAs }

// Maintainer returns the chapter maintainer.
func (ch *Chapter) Maintainer() string { return ch.maintainer }

// LegacySources returns the "refers to" URLs of the original documentation.
func (ch *Chapter) LegacySources() []string { return ch.legacySources }

// Sections returns the subsections of the chapter.
func (ch *Chapter) Sections() []Section { return ch.sections }

// Procedures returns the numbered lists of steps.
func (ch *Chapter) Procedures() []Procedure { return ch.procedures }

// Admonitions returns the note, tip, important, warning and caution boxes.
func (ch *Chapter) Admonitions() []Admonition { return ch.admonitions }

// Xrefs returns the cross references of the chapter.
func (ch *Chapter) Xrefs() []Xref { return ch.xrefs }

// Links returns the external links of the chapter.
func (ch *Chapter) Links() []Link { return ch.links }

// Images returns the images shown in the chapter.
func (ch *Chapter) Images() []Image { return ch.images }

// Includes returns the XIncluded snippet files of the chapter.
func (ch *Chapter) Includes() []string { return ch.includes }

// SetNumber sets the chapter number as published.
func (ch *Chapter) SetNumber(v string) { ch.number = v }

// SetTopicType sets the topic type.
func (ch *Chapter) SetTopicType(v string) { ch.topicType = v }

// SetRenderAs sets how the chapter is rendered.
func (ch *Chapter) SetRenderAs(v string) { ch.renderAs = v }

// SetMaintainer sets the chapter maintainer.
func (ch *Chapter) SetMaintainer(v string) { ch.maintainer = v }

// SetLegacySources replaces the "refers to" URLs of the original documentation.
func (ch *Chapter) SetLegacySources(v []string) { ch.legacySources = v }

// AddLegacySource appends a "refers to" URL of the original documentation.
func (ch *Chapter) AddLegacySource(u string) { ch.legacySources = append(ch.legacySources, u) }

// AddSection appends a subsection of the chapter.
func (ch *Chapter) AddSection(s Section) { ch.sections = append(ch.sections, s) }

// AddProcedure appends a numbered list of steps.
func (ch *Chapter) AddProcedure(p Procedure) { ch.procedures = append(ch.procedures, p) }

// AddAdmonition appends a note, tip, important, warning or caution box.
func (ch *Chapter) AddAdmonition(a Admonition) { ch.admonitions = append(ch.admonitions, a) }

// AddXref appends a cross reference of the chapter.
func (ch *Chapter) AddXref(x Xref) { ch.xrefs = append(ch.xrefs, x) }

// AddLink appends an external link of the chapter.
func (ch *Chapter) AddLink(l Link) { ch.links = append(ch.links, l) }

// AddImage appends an image shown in the chapter.
func (ch *Chapter) AddImage(i Image) { ch.images = append(ch.images, i) }

// AddInclude appends an XIncluded snippet file of the chapter.
func (ch *Chapter) AddInclude(f string) { ch.includes = append(ch.includes, f) }

// chapterMeta is the serialization form of a Chapter; the DocBook-only fields
// are included so the CLI dump keeps the same shape as before unexporting them.
type chapterMeta struct {
	ID            string       `json:"id" yaml:"id"`
	Title         string       `json:"chapter_title" yaml:"chapter_title"`
	Summary       string       `json:"summary,omitempty" yaml:"summary,omitempty"`
	Start         uint         `json:"start" yaml:"start"`
	End           uint         `json:"end" yaml:"end"`
	FilePath      string       `json:"file_path" yaml:"file_path"`
	SourcePath    string       `json:"source_path,omitempty" yaml:"source_path,omitempty"`
	Commands      []string     `json:"commands,omitempty" yaml:"commands,omitempty"`
	Number        string       `json:"number,omitempty" yaml:"number,omitempty"`
	TopicType     string       `json:"topic_type,omitempty" yaml:"topic_type,omitempty"`
	RenderAs      string       `json:"render_as,omitempty" yaml:"render_as,omitempty"`
	Maintainer    string       `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	LegacySources []string     `json:"legacy_sources,omitempty" yaml:"legacy_sources,omitempty"`
	Sections      []Section    `json:"sections,omitempty" yaml:"sections,omitempty"`
	Procedures    []Procedure  `json:"procedures,omitempty" yaml:"procedures,omitempty"`
	Admonitions   []Admonition `json:"admonitions,omitempty" yaml:"admonitions,omitempty"`
	Xrefs         []Xref       `json:"xrefs,omitempty" yaml:"xrefs,omitempty"`
	Links         []Link       `json:"links,omitempty" yaml:"links,omitempty"`
	Images        []Image      `json:"images,omitempty" yaml:"images,omitempty"`
	Includes      []string     `json:"includes,omitempty" yaml:"includes,omitempty"`
}

func (ch *Chapter) chapterMeta() chapterMeta {
	return chapterMeta{ID: ch.ID, Title: ch.Title, Summary: ch.Summary, Start: ch.Start, End: ch.End,
		FilePath: ch.FilePath, SourcePath: ch.SourcePath, Commands: ch.Commands, Number: ch.number,
		TopicType: ch.topicType, RenderAs: ch.renderAs, Maintainer: ch.maintainer,
		LegacySources: ch.legacySources, Sections: ch.sections, Procedures: ch.procedures,
		Admonitions: ch.admonitions, Xrefs: ch.xrefs, Links: ch.links, Images: ch.images, Includes: ch.includes}
}

// MarshalJSON implements json.Marshaler so the DocBook-only fields are still
// serialized even though they are unexported.
func (ch Chapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(ch.chapterMeta())
}

// MarshalYAML implements yaml.Marshaler (gopkg.in/yaml.v3) so the DocBook-only
// fields are still serialized even though they are unexported.
func (ch Chapter) MarshalYAML() (interface{}, error) {
	return ch.chapterMeta(), nil
}
