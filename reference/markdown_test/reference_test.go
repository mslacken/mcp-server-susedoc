// Package markdown_test checks the markdown importer against the reference
// documents: the markdown files in ../markdown are imported as the server
// does and every resulting document is compared with the handcrafted dump in
// this directory whose chapters were written in the same file.
package markdown_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/loader"
)

// markdownDir holds the reference markdown files, relative to this package.
const markdownDir = "../markdown"

// expectedDumps returns the reference json files of this directory by the
// file their first chapter was written in. They are paired with the markdown
// files that way and not by document id, so a wrong id shows up in the diff.
func expectedDumps(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.json")
	require.NoError(t, err)
	dumps := map[string]string{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		var dump struct {
			Document struct {
				Chapters []struct {
					File string `json:"file"`
				} `json:"chapters"`
			} `json:"document"`
		}
		require.NoError(t, json.Unmarshal(raw, &dump), p)
		require.NotEmpty(t, dump.Document.Chapters, "%s has no chapters", p)
		dumps[dump.Document.Chapters[0].File] = string(raw)
	}
	return dumps
}

func TestReferenceMarkdown(t *testing.T) {
	files, err := loader.Find(markdownDir)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no markdown files in %s", markdownDir)
	dumps := expectedDumps(t)

	corpus, err := loader.Load(context.Background(), files)
	require.NoError(t, err)
	require.Len(t, corpus.Documents, len(dumps), "documents imported vs. reference json files")

	for i := range corpus.Documents {
		d := &corpus.Documents[i]
		require.NotEmpty(t, d.Chapters, "document %q has no chapters", d.ID)
		name := d.Chapters[0].File
		t.Run(name, func(t *testing.T) {
			want, ok := dumps[name]
			require.True(t, ok, "no reference json for %s", name)

			var got bytes.Buffer
			require.NoError(t, corpus.DumpDocument(&got, d))
			assert.JSONEq(t, want, got.String())
		})
	}
}

// TestReferenceXrefs checks that the cross references of the reference
// documents resolve to the docs:// URIs of their targets, within the
// Reference Guide and from the Linked Guide into it.
func TestReferenceXrefs(t *testing.T) {
	files, err := loader.Find(markdownDir)
	require.NoError(t, err)
	corpus, err := loader.Load(context.Background(), files)
	require.NoError(t, err)

	xrefs := func(docID, chapterID string) []docparse.XrefTarget {
		doc := corpus.Document(docID)
		require.NotNil(t, doc, docID)
		ch := doc.Chapter(chapterID)
		require.NotNil(t, ch, chapterID)
		return corpus.XrefTargets(doc.ID, ch)
	}
	chapter := func(id, title, uri string) docparse.XrefTarget {
		return docparse.XrefTarget{Linkend: id, Text: title, Resolved: true, DocID: "reference-doc", ChapterID: id, Title: title, Scope: "chapter", URI: uri}
	}
	section := func(id, chapterID, title, uri string) docparse.XrefTarget {
		return docparse.XrefTarget{Linkend: id, Text: title, Resolved: true, DocID: "reference-doc", ChapterID: chapterID, SectionID: id, Title: title, Scope: "section", URI: uri}
	}

	assert.Equal(t, []docparse.XrefTarget{
		chapter("commands", "Commands", "docs://reference-doc/commands"),
		section("json-header-example", "front-matter", "JSON header", "docs://reference-doc/front-matter#json-header-example"),
		section("sections", "document-structure", "Sections", "docs://reference-doc/document-structure#sections"),
		{Linkend: "no-such-target", Resolved: false},
	}, xrefs("reference-doc", "cross-references"))

	assert.Equal(t, []docparse.XrefTarget{
		chapter("front-matter", "Front matter", "docs://reference-doc/front-matter"),
		chapter("commands", "Commands", "docs://reference-doc/commands"),
		section("json-header-example", "front-matter", "JSON header", "docs://reference-doc/front-matter#json-header-example"),
		section("interactive-shells", "commands", "Interactive shells", "docs://reference-doc/commands#interactive-shells"),
	}, xrefs("linked-reference-doc", "links-to-other-documents"))
}
