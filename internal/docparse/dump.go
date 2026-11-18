package docparse

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// DumpVersion is the version of the dump format, for future compatibility.
const DumpVersion = 1

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

type dumpCorpus struct {
	Version   int                      `json:"version"`
	Documents []dumpDocument           `json:"documents"`
	Glossary  map[string]GlossaryEntry `json:"glossary"`
}

// Dump writes the corpus to w in the dump format, see Load.
func (c *Corpus) Dump(w io.Writer) error {
	out := dumpCorpus{Version: DumpVersion, Documents: []dumpDocument{}, Glossary: c.Glossary}
	for i := range c.Documents {
		d := &c.Documents[i]
		doc := dumpDocument{
			ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
			License: d.License, Chapters: make([]dumpChapter, 0, len(d.Chapters)),
		}
		for j := range d.Chapters {
			ch := &d.Chapters[j]
			doc.Chapters = append(doc.Chapters, dumpChapter{
				ID: ch.ID, Title: ch.Title, Summary: ch.Summary, Start: ch.Start, End: ch.End,
				FilePath: ch.FilePath, SourcePath: ch.SourcePath, Commands: ch.Commands,
				Sections: ch.Sections, Xrefs: ch.Xrefs,
				Text: c.Text(ch), Prose: ch.Prose,
			})
		}
		out.Documents = append(out.Documents, doc)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Load reads a corpus written by Dump from r and adds its documents, glossary
// and lines to c, as Add does. Finish must be called once c is complete.
func (c *Corpus) Load(r io.Reader) error {
	var in dumpCorpus
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return err
	}
	if in.Version != DumpVersion {
		return fmt.Errorf("unknown dump format version %d", in.Version)
	}
	p := Part{Lines: map[string][]string{}}
	for i := range in.Documents {
		d := &in.Documents[i]
		doc := Document{
			ID: d.ID, Title: d.Title, Summary: d.Summary, Distro: d.Distro, Product: d.Product,
			License: d.License, Chapters: make([]Chapter, 0, len(d.Chapters)),
		}
		for j := range d.Chapters {
			ch := &d.Chapters[j]
			lines := strings.Split(ch.Text, "\n")
			// the document index is unique within c, unlike the document id
			key := fmt.Sprintf("dump\x00%d\x00%d", len(c.Documents)+i, j)
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
	}
	for _, e := range in.Glossary {
		p.Glossary = append(p.Glossary, e)
	}
	c.Add(p)
	return nil
}
