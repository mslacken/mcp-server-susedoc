package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mcp-server-susedoc/internal/docparse/loader"
	"mcp-server-susedoc/internal/docparse/markdown"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sample = `# Combined Documentation

---

<!-- Source: guide/index.md -->

---
title: Guide
description: A sample guide
distro: Sample Product
version: "1.0"
---

# Guide

## Setup

The CIB holds the configuration of every node.

` + "```" + `
# crm configure show
` + "```" + `

## Setup

Second chapter with the same title.

---

<!-- Source: guide/ha-glossary.md -->

# Glossary

CIB (cluster information base)

The cluster configuration.

node

A cluster member.

DC (designated coordinator)

The coordinator.
`

func TestReadDocument(t *testing.T) {
	c := markdown.BuildDocuments(markdown.SplitSources([]byte(sample), "combined.md"))
	require.Len(t, c.Documents, 1)
	require.Len(t, c.Glossary, 3)

	tocOnly, err := BuildReadDocument(c, "guide", "")
	require.NoError(t, err)
	require.Len(t, tocOnly.TOC, 2)
	assert.Equal(t, "setup", tocOnly.TOC[0].ID)
	assert.Equal(t, "setup-2", tocOnly.TOC[1].ID)
	assert.Nil(t, tocOnly.Chapter, "chapter in table-of-contents result")

	out, err := BuildReadDocument(c, "guide", "setup")
	require.NoError(t, err)
	assert.Empty(t, out.TOC, "table of contents with a chapter")
	ch := out.Chapter
	require.NotNil(t, ch)
	assert.True(t, strings.HasPrefix(ch.Markdown, "## Setup {#setup}\n\nThe CIB"), "markdown = %q", ch.Markdown)
	assert.Equal(t, "guide/index.md", ch.File)
	var terms []string
	for _, g := range ch.Glossary {
		terms = append(terms, g.Term)
	}
	assert.ElementsMatch(t, []string{"CIB (cluster information base)", "node"}, terms)
	assert.Equal(t, []string{"crm configure show"}, ch.Commands)
	second, err := BuildReadDocument(c, "guide", "setup-2")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(second.Chapter.Markdown, "## Setup {#setup-2}\n"), "markdown = %q", second.Chapter.Markdown)
	_, err = BuildReadDocument(c, "guide", "nope")
	assert.Error(t, err, "unknown chapter")
}

// docbookFiles is a minimal DocBook assembly tree with all the structured
// information the DocBook parser records only for the CLI dump.
var docbookFiles = map[string]string{
	"DC-SLES-demo": "MAIN=\"demo.asm.xml\"\nSRC_DIR=\"articles\"\nPROFOS=\"sles\"\n",
	"common/entities.ent": `<!ENTITY prompt.sudo "<prompt xmlns='http://docbook.org/ns/docbook'>&gt; </prompt><command xmlns='http://docbook.org/ns/docbook'>sudo</command> ">
<!ENTITY productname "<phrase xmlns='http://docbook.org/ns/docbook'><phrase os='sles'>Demo Server</phrase><phrase os='other'>Other</phrase></phrase>">`,
	"articles/demo.asm.xml": `<?xml version="1.0"?>
<!DOCTYPE assembly [ <!ENTITY % e SYSTEM "../common/entities.ent"> %e; ]>
<assembly xmlns="http://docbook.org/ns/docbook">
  <resources>
    <resource href="../tasks/task.xml" xml:id="_task"/>
    <resource href="../references/glossary.xml" xml:id="_glossary"/>
  </resources>
  <structure xml:id="demo">
    <merge>
      <title>Demo</title><subtitle>Sub</subtitle>
      <revhistory><revision><date>2026-01-01</date><revdescription><para>Initial</para></revdescription></revision></revhistory>
      <meta name="maintainer" content="tux@example.com"/>
      <meta name="productname"><productname version="1.0">&productname;</productname></meta>
      <dm:docmanager xmlns:dm="urn:x-suse:ns:docmanager"><dm:bugtracker><dm:url>https://bugs</dm:url></dm:bugtracker><dm:translation>yes</dm:translation></dm:docmanager>
    </merge>
    <module resourceref="_task"/>
    <module resourceref="_glossary" renderas="glossary"/>
  </structure>
</assembly>`,
	"tasks/task.xml": `<?xml version="1.0"?>
<!DOCTYPE topic [ <!ENTITY % e SYSTEM "../common/entities.ent"> %e; ]>
<!-- refers to legacy doc: https://github.com/SUSE/doc-sle/blob/main/xml/x.xml -->
<topic xml:id="task" role="task" xmlns="http://docbook.org/ns/docbook" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xi="http://www.w3.org/2001/XInclude">
  <info><title>Install</title><meta name="maintainer" content="geeko@example.com"/></info>
  <para>The CIB, see <xref linkend="sec"/> and <link xlink:href="https://example.com">docs</link>.</para>
  <procedure><step><para>Run:</para><screen>&prompt.sudo;zypper in demo</screen></step></procedure>
  <note><para>Careful.</para></note>
  <xi:include href="../snippets/tip.xml"/>
  <mediaobject><imageobject><imagedata fileref="a.png"/></imageobject></mediaobject>
  <section xml:id="sec"><title>Section</title><para>Text.</para></section>
</topic>`,
	"snippets/tip.xml": `<tip xmlns="http://docbook.org/ns/docbook"><para>Included tip.</para></tip>`,
	"references/glossary.xml": `<?xml version="1.0"?>
<topic xml:id="glossary" xmlns="http://docbook.org/ns/docbook">
  <glossentry><glossterm>CIB (cluster information base)</glossterm><glossdef><para>The configuration.</para></glossdef></glossentry>
</topic>`,
}

// TestDocBookFieldsNotExposed checks that the structured DocBook information
// shows up in the CLI dump but in none of the MCP tool or resource results.
func TestDocBookFieldsNotExposed(t *testing.T) {
	root := t.TempDir()
	for name, content := range docbookFiles {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	files, err := loader.Find(root)
	require.NoError(t, err)
	c, err := loader.Load(files)
	require.NoError(t, err)
	hidden := []string{"source", "profile", "subtitle", "meta", "revisions", "bugtracker", "translation",
		"topic_type", "render_as", "maintainer", "legacy_sources", "sections", "procedures", "admonitions",
		"xrefs", "links", "images", "includes"}
	// the format specific extras are not serialized with the corpus, so
	// marshal them alongside the documents to see that the parser filled them
	var extras []any
	for _, d := range c.Documents {
		extras = append(extras, d.Extras)
		for _, ch := range d.Chapters {
			extras = append(extras, ch.Extra)
		}
	}
	dump, err := json.Marshal(map[string]any{"documents": c.Documents, "extras": extras})
	require.NoError(t, err)
	for _, k := range hidden {
		assert.Contains(t, string(dump), `"`+k+`":`, "parsed corpus lacks %q", k)
	}

	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	_, err = NewServer(c).Connect(ctx, st, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()
	var outputs []string
	for name, args := range map[string]map[string]any{
		"list_documents": {"product": "Demo Server", "version": "1.0"},
		"read_document":  {"doc_id": "sles-demo", "chapter_id": "task"},
		"search_docs":    {"query": "install"},
		"find_command":   {"pattern": "zypper"},
		"define_term":    {"term": "CIB"},
		"list_glossary":  {},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, err, name)
		require.False(t, res.IsError, "%s: %+v", name, res)
		data, _ := json.Marshal(res)
		outputs = append(outputs, string(data))
	}
	for _, uri := range []string{"docs://index", "docs://glossary", "docs://sles-demo", "docs://sles-demo/task", "docs://sles-demo/task#sec"} {
		res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		require.NoError(t, err, uri)
		data, _ := json.Marshal(res)
		outputs = append(outputs, string(data))
	}
	all := strings.Join(outputs, "\n")
	for _, k := range hidden {
		assert.NotContains(t, all, `"`+k+`":`, "MCP output exposes %q", k)
		assert.NotContains(t, all, `\"`+k+`\":`, "MCP output exposes %q", k)
	}
	for _, want := range []string{"sudo zypper in demo", "Demo Server", "The configuration.", "## 1 Install {#task}",
		`"uri":"docs://sles-demo/task#sec"`, `## References\n\n- [Section](docs://sles-demo/task#sec)`} {
		assert.Contains(t, all, want)
	}
}
