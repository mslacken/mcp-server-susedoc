package docparse

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDumpLoad(t *testing.T) {
	c := NewCorpus()
	c.Add(Part{
		Documents: []Document{{
			ID: "guide", Title: "Guide", Distro: Distro{Name: "SLES", Versions: []string{"16.0"}},
			Chapters: []Chapter{{
				ID: "intro", Title: "Intro", Start: 2, End: 3, File: "guide.md",
				Commands: []string{"zypper in foo"}, LinesKey: "guide.md", Prose: "Install foo.",
			}},
		}},
		Glossary: []GlossaryEntry{{Term: "Foo", WhatIs: "A package."}, {Term: "Bar", WhatIs: "Not mentioned."}},
		Lines:    map[string][]string{"guide.md": {"preamble", "# Intro", "Install foo."}},
	})
	c.Finish()

	var buf bytes.Buffer
	require.NoError(t, c.DumpDocument(&buf, &c.Documents[0]))
	assert.NotContains(t, buf.String(), `"prose"`)
	got := NewCorpus()
	require.NoError(t, got.Load(&buf))
	got.Finish()

	ch := &got.Documents[0].Chapters[0]
	assert.Equal(t, "# Intro\nInstall foo.", got.Text(ch))
	assert.Equal(t, "# Intro\nInstall foo.", ch.Prose, "without ProseOf the markdown is the prose")
	assert.Equal(t, []string{"zypper in foo"}, ch.Commands)
	assert.Equal(t, c.Documents[0].Distro, got.Documents[0].Distro)
	if e := got.DefineTerm("foo", nil); assert.Len(t, e, 1) {
		assert.Equal(t, "A package.", e[0].WhatIs)
	}
	assert.Empty(t, got.DefineTerm("bar", nil), "unmentioned glossary term dumped")
}
