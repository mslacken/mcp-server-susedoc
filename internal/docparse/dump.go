package docparse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"mcp-server-susedoc/internal/debug"

	"golang.org/x/sync/errgroup"
)

var debugf = debug.New("docparse")

// DumpVersion is the version of the dump format, for future compatibility.
// Version 1 held the whole corpus in one file, version 2 holds one document
// per file, version 3 merges the chapter's file_path and source_path into
// file, drops its start and end lines and holds xrefs as plain linkends.
const DumpVersion = 3

// The dump format holds the MCP surface of every document and chapter (id,
// title, summary, distro, product, file, commands, sections, xrefs, license), plus
// each chapter's markdown, so that a corpus read back from it is self
// sufficient and can be served without the original sources. The prose of a
// chapter is not part of it, it is recreated from the markdown, see ProseOf.
// Format specific extras (like the DocBook provenance) are not part of it either.

// ProseOf returns the running text of a chapter's markdown lines, without
// code, for glossary matching. It is set by the markdown package; if it is
// nil, the whole markdown is taken as prose.
var ProseOf func(lines []string) string

type dumpChapter struct {
	ID       string    `json:"id"`
	Title    string    `json:"chapter_title"`
	Summary  string    `json:"summary,omitempty"`
	File     string    `json:"file,omitempty"`
	Commands []string  `json:"commands,omitempty"`
	Sections []Section `json:"sections,omitempty"`
	Xrefs    []string  `json:"xrefs,omitempty"`
	Text     string    `json:"text"`
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
// it defines or its chapters mention.
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
	// every document gets its own slot, so all failures are reported and not
	// just the first one, as errgroup would do
	errs := make([]error, len(c.Documents))
	var g errgroup.Group
	// bound the number of files open at once
	g.SetLimit(runtime.GOMAXPROCS(0))
	for i := range c.Documents {
		d := &c.Documents[i]
		if d.ID == "" || d.ID != filepath.Base(d.ID) || strings.HasPrefix(d.ID, ".") {
			errs[i] = fmt.Errorf("document id %q is not usable as file name", d.ID)
			continue
		}
		g.Go(func() error {
			f, err := os.Create(filepath.Join(dir, d.ID+".json"))
			if err != nil {
				errs[i] = err
				return nil
			}
			debugf("dumping %s", d.ID)
			err = c.DumpDocument(f, d)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", d.ID, err)
			}
			return nil
		})
	}
	_ = g.Wait()
	return errors.Join(errs...)
}

// DumpDocument writes the document d of the corpus to w in the dump format,
// together with the glossary entries it defines or its chapters mention, in
// term order, so that the
// file can be read back on its own, see Load. Finish must have been called.
func (c *Corpus) DumpDocument(w io.Writer, d *Document) error {
	out := dumpFile{Version: DumpVersion, Document: dumpDocument{
		ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
		License: d.License, Chapters: make([]dumpChapter, 0, len(d.Chapters)),
	}}
	var terms []string
	for _, e := range d.Glossary {
		terms = append(terms, e.Term)
	}
	for j := range d.Chapters {
		ch := &d.Chapters[j]
		out.Document.Chapters = append(out.Document.Chapters, dumpChapter{
			ID: ch.ID, Title: ch.Title, Summary: ch.Summary, File: ch.File,
			Commands: ch.Commands, Sections: ch.Sections, Xrefs: ch.Xrefs,
			Text: c.Text(ch),
		})
		terms = append(terms, c.termsIn(ch.Prose)...)
	}
	slices.SortFunc(terms, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	terms = slices.CompactFunc(terms, strings.EqualFold)
	out.Glossary = c.GlossaryFor(terms)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Import reads the dump files at path into the corpus and returns their names:
// path itself if it is a file, and otherwise every *.json file in path and,
// recursively, in its subdirectories, following symbolic links. The files are
// read in parallel, into their own parts, and added in file order, so the
// document order and the first definition winning for a glossary term stay
// deterministic. Files that cannot be read are skipped: the corpus holds what
// was read and the error, if any, joins the reasons the others were skipped.
// Finish must be called once the corpus is complete.
func (c *Corpus) Import(path string) ([]string, error) {
	files, err := dumpFiles(path, map[string]bool{})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no dump files in %s", path)
	}
	parts := make([]Part, len(files))
	errs := make([]error, len(files))
	var g errgroup.Group
	// bound the number of files open at once
	g.SetLimit(runtime.GOMAXPROCS(0))
	for i, name := range files {
		g.Go(func() error {
			f, err := os.Open(name)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", name, err)
				return nil
			}
			parts[i], err = loadFile(f, i)
			_ = f.Close()
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", name, err)
			}
			return nil
		})
	}
	_ = g.Wait()
	// a file that cannot be read leaves an empty part
	for _, p := range parts {
		c.Add(p)
	}
	return files, errors.Join(errs...)
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
	p, err := loadFile(r, len(c.Documents))
	if err != nil {
		return err
	}
	c.Add(p)
	return nil
}

// loadFile reads one dump file written by DumpDocument from r into a Part,
// with the document offset giving each chapter's lines a unique key. Adding
// the Part to a corpus is the caller's job, so it can be done after the files
// are all read, in order.
func loadFile(r io.Reader, offset int) (Part, error) {
	var in dumpFile
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return Part{}, err
	}
	if in.Version != DumpVersion {
		return Part{}, fmt.Errorf("unknown dump format version %d", in.Version)
	}
	d := &in.Document
	doc := Document{
		ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
		License: d.License, Glossary: in.Glossary, Chapters: make([]Chapter, 0, len(d.Chapters)),
	}
	p := Part{Lines: map[string][]string{}, Glossary: in.Glossary}
	for j := range d.Chapters {
		ch := &d.Chapters[j]
		lines := strings.Split(ch.Text, "\n")
		// the document offset is unique within the corpus, unlike the document id
		key := fmt.Sprintf("dump\x00%d\x00%d", offset, j)
		p.Lines[key] = lines
		prose := ch.Text
		if ProseOf != nil {
			prose = ProseOf(lines)
		}
		doc.Chapters = append(doc.Chapters, Chapter{
			ID: ch.ID, Title: ch.Title, Summary: ch.Summary, File: ch.File,
			Commands: ch.Commands, Sections: ch.Sections, Xrefs: ch.Xrefs,
			LinesKey: key, Start: 1, End: uint(len(lines)), Prose: prose,
		})
	}
	p.Documents = append(p.Documents, doc)
	return p, nil
}
