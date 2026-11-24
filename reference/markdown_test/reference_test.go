// Package markdown_test checks the markdown importer against the reference
// documents: the markdown files in ../markdown are imported as the server
// does and every resulting document is compared with the handcrafted dump in
// this directory whose chapters were written in the same file.
package markdown_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	corpus, err := loader.Load(files)
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
