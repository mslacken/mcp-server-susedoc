package docparse

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"mcp-server-susedoc/internal/debug"
)

var debugf = debug.New("docparse")

// DumpVersion is the version of the dump format, for future compatibility.
// Version 1 held the whole corpus in one file, version 2 holds one document
// per file.
const DumpVersion = 2

// The dump format holds the MCP surface of every document and chapter (id,
// title, summary, distro, product, commands, sections, xrefs, license), plus
// each chapter's markdown and prose, so that a corpus read back from it is self
// sufficient and can be served without the original sources. Format specific
// extras (like the DocBook provenance) are not part of it.

type dumpChapter struct {
	ID         string    `json:"id"`
	Title      string    `json:"chapter_title"`
	Summary    string    `json:"summary,omitempty"`
	Start      uint      `json:"start"`
	End        uint      `json:"end"`
	FilePath   string    `json:"file_path"`
	SourcePath string    `json:"source_path,omitempty"`
	Commands   []string  `json:"commands,omitempty"`
	Sections   []Section `json:"sections,omitempty"`
	Xrefs      []Xref    `json:"xrefs,omitempty"`
	Text       string    `json:"text"`
	Prose      string    `json:"prose,omitempty"`
}

type dumpDocument struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Summary  string        `json:"summary"`
	Distro   Distro        `json:"distro"`
	Product  Product       `json:"product"`
	Chapters []dumpChapter `json:"chapters"`
	License  string        `json:"license,omitempty"`
}

// dumpFile is one file of a dump: a single document with the glossary entries
// its chapters mention.
type dumpFile struct {
	Version  int             `json:"version"`
	Document dumpDocument    `json:"document"`
	Glossary []GlossaryEntry `json:"glossary,omitempty"`
}

// Export writes every document of the corpus to dir as <id>.json in the dump
// format, see DumpDocument. Finish must have been called.
func (c *Corpus) Export(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i := range c.Documents {
		d := &c.Documents[i]
		if d.ID == "" || d.ID != filepath.Base(d.ID) || strings.HasPrefix(d.ID, ".") {
			return fmt.Errorf("document id %q is not usable as file name", d.ID)
		}
		f, err := os.Create(filepath.Join(dir, d.ID+".json"))
		if err != nil {
			return err
		}
		debugf("dumping %s", d.ID)
		err = c.DumpDocument(f, d)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DumpDocument writes the document d of the corpus to w in the dump format,
// together with the glossary entries mentioned in its chapters, so that the
// file can be read back on its own, see Load. Finish must have been called.
func (c *Corpus) DumpDocument(w io.Writer, d *Document) error {
	out := dumpFile{Version: DumpVersion, Document: dumpDocument{
		ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
		License: d.License, Chapters: make([]dumpChapter, 0, len(d.Chapters)),
	}}
	var terms []string
	for j := range d.Chapters {
		ch := &d.Chapters[j]
		out.Document.Chapters = append(out.Document.Chapters, dumpChapter{
			ID: ch.ID, Title: ch.Title, Summary: ch.Summary, Start: ch.Start, End: ch.End,
			FilePath: ch.FilePath, SourcePath: ch.SourcePath, Commands: ch.Commands,
			Sections: ch.Sections, Xrefs: ch.Xrefs,
			Text: c.Text(ch), Prose: ch.Prose,
		})
		terms = append(terms, c.termsIn(ch.Prose)...)
	}
	slices.Sort(terms)
	out.Glossary = c.GlossaryFor(slices.Compact(terms))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Import reads the dump files at path into the corpus and returns their names:
// path itself if it is a file, and otherwise every *.json file in path and,
// recursively, in its subdirectories, following symbolic links. Finish must be
// called once the corpus is complete.
func (c *Corpus) Import(path string) ([]string, error) {
	files, err := dumpFiles(path, map[string]bool{})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no dump files in %s", path)
	}
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		err = c.Load(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return files, nil
}

// dumpFiles returns the dump files at path, see Import, in lexical order.
// seen holds the directories already visited, to break symbolic link loops.
func dumpFiles(path string, seen map[string]bool) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if seen[real] {
		return nil, nil
	}
	seen[real] = true
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := filepath.Join(path, e.Name())
		info, err := os.Stat(name)
		if err != nil {
			continue // dangling symbolic link
		}
		if info.IsDir() {
			sub, err := dumpFiles(name, seen)
			if err != nil {
				return nil, err
			}
			files = append(files, sub...)
		} else if strings.HasSuffix(e.Name(), ".json") {
			files = append(files, name)
		}
	}
	return files, nil
}

// Load reads one dump file written by DumpDocument from r and adds its
// document, glossary and lines to c, as Add does. Finish must be called once c
// is complete.
func (c *Corpus) Load(r io.Reader) error {
	var in dumpFile
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return err
	}
	if in.Version != DumpVersion {
		return fmt.Errorf("unknown dump format version %d", in.Version)
	}
	d := &in.Document
	doc := Document{
		ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
		License: d.License, Chapters: make([]Chapter, 0, len(d.Chapters)),
	}
	p := Part{Lines: map[string][]string{}, Glossary: in.Glossary}
	for j := range d.Chapters {
		ch := &d.Chapters[j]
		lines := strings.Split(ch.Text, "\n")
		// the document index is unique within c, unlike the document id
		key := fmt.Sprintf("dump\x00%d\x00%d", len(c.Documents), j)
		p.Lines[key] = lines
		doc.Chapters = append(doc.Chapters, Chapter{
			ID: ch.ID, Title: ch.Title, Summary: ch.Summary,
			Start: 1, End: uint(len(lines)),
			FilePath: ch.FilePath, SourcePath: ch.SourcePath, Commands: ch.Commands,
			Sections: ch.Sections, Xrefs: ch.Xrefs,
			LinesKey: key, Prose: ch.Prose,
		})
	}
	p.Documents = append(p.Documents, doc)
	c.Add(p)
	return nil
}
