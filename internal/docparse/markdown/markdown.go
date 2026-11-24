// Package markdown parses markdown files, either a directory tree of them or
// a single (possibly combined, see en-us/combine.sh) file, into a corpus.
package markdown

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"mcp-server-susedoc/internal/debug"
	"mcp-server-susedoc/internal/docparse"

	"gopkg.in/yaml.v3"
)

// warnf reports content that is dropped.
var warnf = debug.NewWarn("markdown")

// ReadFile reads the markdown file p. root is the directory p was found in,
// or "" if p was named directly. A combined file (see combine.sh) is split
// into its sources if it was named directly and skipped otherwise, as its
// content is already in the tree.
func ReadFile(p, root string) ([]Segment, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if root == "" {
		return SplitSources(data, p), nil
	}
	if isCombined(data) {
		warnf("skipping combined file %s", p)
		return nil, nil
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return nil, err
	}
	s := newSegment(filepath.ToSlash(rel), p, strings.Split(string(data), "\n"))
	s.group = filepath.Dir(p)
	return []Segment{s}, nil
}

func isCombined(data []byte) bool {
	for _, ln := range strings.Split(string(data), "\n") {
		if sourceMarker.MatchString(ln) {
			return true
		}
	}
	return false
}

// documentID is the name of the directory holding the guide, e.g.
// "HA-crmsh-introduction", or the file name for a lone file.
func documentID(file string) string {
	if dir := path.Base(path.Dir(file)); dir != "." && dir != "/" {
		return dir
	}
	return strings.TrimSuffix(path.Base(file), path.Ext(file))
}

// Segment is one source file. For a combined file (see en-us/combine.sh) every
// `<!-- Source: ... -->` marker starts a new segment; lines[0] is line 1 of the
// original file.
type Segment struct {
	file  string // original source file (from `<!-- Source: ... -->` or the input name)
	name  string // file the content was parsed from (the combined file, or input name if not combined)
	meta  map[string]any
	group string // segments with the same group form one document (one directory per guide)
	lines []string
	body  int // index of the first line after the front matter
}

// key is the key of the lines of s, unique across files and roots.
func (s Segment) key() string {
	return s.name + "\x00" + s.file
}

var sourceMarker = regexp.MustCompile(`^<!-- Source: (.+?) -->\s*$`)

// SplitSources splits the markdown in data into one segment per <!-- Source: -->
// marker. For non combined input it returns a single segment named name.
func SplitSources(data []byte, name string) []Segment {
	lines := strings.Split(string(data), "\n")
	var segs []Segment
	file, start, combined := name, 0, false
	for i, ln := range lines {
		m := sourceMarker.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		// the preamble before the first marker is just the combine header
		if combined {
			segs = append(segs, newSourceSegment(file, name, lines[start:i]))
		}
		combined = true
		file = m[1]
		// combine.sh writes a blank line after the marker
		start = min(i+2, len(lines))
	}
	return append(segs, newSourceSegment(file, name, lines[start:]))
}

// newSourceSegment is a segment of the file name, grouped by the directory of
// its original source file.
func newSourceSegment(file, name string, lines []string) Segment {
	s := newSegment(file, name, lines)
	s.group = name + "\x00" + path.Dir(file)
	return s
}

func newSegment(file, name string, lines []string) Segment {
	// drop the "\n---\n\n" separator combine.sh puts before the next marker
	for len(lines) > 0 {
		t := strings.TrimSpace(lines[len(lines)-1])
		if t != "" && t != "---" {
			break
		}
		lines = lines[:len(lines)-1]
	}
	s := Segment{file: file, name: name, lines: lines}
	if len(lines) > 0 && lines[0] == "---" {
		for j := 1; j < len(lines); j++ {
			if lines[j] == "---" {
				if yaml.Unmarshal([]byte(strings.Join(lines[1:j], "\n")), &s.meta) == nil {
					s.body = j + 1
				}
				break
			}
		}
	}
	return s
}

type heading struct {
	level int
	text  string
	idx   int // index into segment.lines
}

var (
	reATX   = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	reFence = regexp.MustCompile("^\\s*(```|~~~)")
)

func headings(s Segment) []heading {
	var hs []heading
	fenced := false
	for i := s.body; i < len(s.lines); i++ {
		if reFence.MatchString(s.lines[i]) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := reATX.FindStringSubmatch(s.lines[i]); m != nil {
			hs = append(hs, heading{level: len(m[1]), text: m[2], idx: i})
		}
	}
	return hs
}

var (
	reHistory     = regexp.MustCompile(`(?i)^revision history`)
	reChapterNum  = regexp.MustCompile(`^\d+(?:\.\d+)*\s*`)
	reTitleSuffix = regexp.MustCompile(`\s*\|.*$`)
	reAppliesTo   = regexp.MustCompile(`^(?:Applies to )?(.+?) (\d+(?:\.\d+)*(?: SP\d+)?)$`)
	// explicit heading id: "## JSON header {#json-header}"
	reHeadingID = regexp.MustCompile(`\s*\{#([^}\s]+)\}$`)
	// leading section number: "3.2.1 Numbered sections"
	reSectionNum = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s+`)
)

func metaString(meta map[string]any, key string) string {
	if v, ok := meta[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return strings.Join(strings.Fields(s), " ")
		}
	}
	return ""
}

// metaStrings returns the value of key in meta as a list: a single string or
// the strings of a list.
func metaStrings(meta map[string]any, key string) []string {
	switch v := meta[key].(type) {
	case string:
		if s := metaString(meta, key); s != "" {
			return []string{s}
		}
	case []any:
		var out []string
		for _, e := range v {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.Join(strings.Fields(s), " "))
			}
		}
		return out
	}
	return nil
}

// BuildDocuments turns segments into a finished corpus, see Build.
func BuildDocuments(segs []Segment) *docparse.Corpus {
	c := docparse.NewCorpus()
	c.Add(Build(segs))
	c.Finish()
	return c
}

// Build groups segments by directory (one directory per guide) and turns each
// group into a Document. The glossaries of all guides are merged.
func Build(segs []Segment) docparse.Part {
	var order []string
	groups := map[string][]Segment{}
	for _, s := range segs {
		if _, ok := groups[s.group]; !ok {
			order = append(order, s.group)
		}
		groups[s.group] = append(groups[s.group], s)
	}
	p := docparse.Part{Lines: map[string][]string{}}
	for _, s := range segs {
		p.Lines[s.key()] = s.lines
	}
	for _, g := range order {
		doc := buildDocument(groups[g])
		if doc.ID == "" {
			doc.ID = documentID(groups[g][0].file)
		}
		p.Documents = append(p.Documents, doc)
		p.Glossary = append(p.Glossary, doc.Glossary...)
	}
	return p
}

// buildDocument turns the segments of one directory into a Document. Its id
// is only set if a front matter names it.
func buildDocument(segs []Segment) docparse.Document {
	doc := docparse.Document{Chapters: []docparse.Chapter{}}
	var glossary []docparse.GlossaryEntry
	var legal, licenseFile []string
	var description string
	for _, s := range segs {
		addDistro(&doc, s)
		if doc.ID == "" {
			doc.ID = metaString(s.meta, "id")
		}
		hs := headings(s)
		first := ""
		if len(hs) > 0 {
			first = hs[0].text
		}
		base := strings.ToLower(path.Base(s.file))
		switch {
		case strings.Contains(base, "license") || docparse.IsLicenseTitle(first):
			licenseFile = append(licenseFile, s.lines[s.body:]...)
		case strings.Contains(base, "glossary"):
			glossary = append(glossary, parseGlossary(s.lines[s.body:])...)
		case strings.HasPrefix(base, "rh-") || reHistory.MatchString(first):
		default:
			if doc.Title == "" {
				description = metaString(s.meta, "description")
				doc.Title = reTitleSuffix.ReplaceAllString(metaString(s.meta, "title"), "")
				for _, h := range hs {
					if h.level == 1 {
						doc.Title = h.text
						break
					}
				}
			}
			l, g := addChapters(&doc, s, hs)
			legal = append(legal, l...)
			glossary = append(glossary, g...)
		}
	}
	doc.Summary = description
	if doc.Summary == "" && len(doc.Chapters) > 0 {
		doc.Summary = doc.Chapters[0].Summary
	}
	doc.License = docparse.License(strings.Join(legal, "\n"), strings.Join(licenseFile, "\n"))
	ids := map[string]int{}
	for i := range doc.Chapters {
		doc.Chapters[i].ID = docparse.UniqueID(ids, docparse.Slug(doc.Chapters[i].Title))
	}
	doc.Glossary = glossary
	return doc
}

// addDistro sets the distribution and product of doc from the front matter of
// s, see addVersions.
func addDistro(doc *docparse.Document, s Segment) {
	distro, versions := metaString(s.meta, "distro"), metaStrings(s.meta, "version")
	if distro == "" || len(versions) == 0 {
		// fall back to the "Applies to DISTRO VERSION" line below the front matter
		for i := s.body; i < len(s.lines) && i < s.body+4; i++ {
			if m := reAppliesTo.FindStringSubmatch(strings.TrimSpace(s.lines[i])); m != nil && strings.HasPrefix(s.lines[i], "Applies to") {
				distro, versions = m[1], []string{m[2]}
				break
			}
		}
	}
	addVersions(&doc.Distro.Name, &doc.Distro.Versions, distro, versions)
	addVersions(&doc.Product.Name, &doc.Product.Versions, metaString(s.meta, "product"), metaStrings(s.meta, "product-version"))
}

// addVersions sets *name to newName unless it is set already and adds the
// new versions to *versions.
func addVersions(name *string, versions *[]string, newName string, newVersions []string) {
	if *name == "" {
		*name = newName
	}
	for _, v := range newVersions {
		if !slices.Contains(*versions, v) {
			*versions = append(*versions, v)
		}
	}
}

// addChapters appends one Chapter per top level section of s and returns the
// text of the legal notice section(s) and the glossary sections, which are not
// treated as chapters.
func addChapters(doc *docparse.Document, s Segment, hs []heading) ([]string, []docparse.GlossaryEntry) {
	level, h1 := 2, 0
	for _, h := range hs {
		if h.level == 1 {
			h1++
		}
	}
	if h1 > 1 {
		level = 1
	}
	var legal []string
	var glossary []docparse.GlossaryEntry
	for i, h := range hs {
		if h.level != level {
			continue
		}
		end := len(s.lines)
		for _, n := range hs[i+1:] {
			if n.level <= level {
				end = n.idx
				break
			}
		}
		for end > h.idx+1 && strings.TrimSpace(s.lines[end-1]) == "" {
			end--
		}
		section := s.lines[h.idx:end]
		title := reChapterNum.ReplaceAllString(h.text, "")
		switch {
		case docparse.IsLegalNotice(title):
			legal = append(legal, section...)
			continue
		case glossaryHeading.MatchString(title):
			glossary = append(glossary, parseGlossary(section)...)
			continue
		}
		a := Analyze(section)
		var sections []docparse.Section
		for _, n := range hs[i+1:] {
			if n.idx >= end {
				break
			}
			sections = append(sections, newSection(n, n.level-level))
		}
		doc.Chapters = append(doc.Chapters, docparse.Chapter{
			Title:    title,
			File:     s.file,
			Commands: a.Commands,
			Sections: sections,
			Xrefs:    a.Xrefs,
			Summary:  a.Lead,
			LinesKey: s.key(),
			Start:    uint(h.idx + 1),
			End:      uint(end),
			Prose:    a.Prose,
		})
	}
	return legal, glossary
}

// newSection returns the section of the heading h, level deep below the
// chapter heading. Its id is the explicit {#id} of the heading, or else the
// slug of its title without the section number.
func newSection(h heading, level int) docparse.Section {
	sec := docparse.Section{Title: h.text, Level: level}
	if m := reHeadingID.FindStringSubmatch(sec.Title); m != nil {
		sec.ID = m[1]
		sec.Title = reHeadingID.ReplaceAllString(sec.Title, "")
	}
	if m := reSectionNum.FindStringSubmatch(sec.Title); m != nil {
		sec.Number = m[1]
		sec.Title = sec.Title[len(m[0]):]
	}
	if sec.ID == "" {
		sec.ID = docparse.Slug(sec.Title)
	}
	return sec
}

func parseGlossary(lines []string) []docparse.GlossaryEntry {
	children := parseNodes(lines)
	marker := node{Type: "Heading", Content: map[string]any{"text": "glossary"}}
	var out []docparse.GlossaryEntry
	for _, c := range collapseGlossary(append([]node{marker}, children...)) {
		if c.Type == "GlossaryEntry" {
			out = append(out, docparse.GlossaryEntry{Term: textOf(c, "term"), WhatIs: textOf(c, "definition")})
		}
	}
	return out
}
