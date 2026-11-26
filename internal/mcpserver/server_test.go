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

	tocOnly, err := BuildReadDocument(c, ReadDocumentIn{DocID: "guide", ChapterID: "", SectionID: ""}, 0)
	require.NoError(t, err)
	require.Len(t, tocOnly.TOC, 2)
	assert.Equal(t, "setup", tocOnly.TOC[0].ID)
	assert.Equal(t, "setup-2", tocOnly.TOC[1].ID)
	assert.Nil(t, tocOnly.Chapter, "chapter in table-of-contents result")

	out, err := BuildReadDocument(c, ReadDocumentIn{DocID: "guide", ChapterID: "setup", SectionID: ""}, 0)
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
	second, err := BuildReadDocument(c, ReadDocumentIn{DocID: "guide", ChapterID: "setup-2", SectionID: ""}, 0)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(second.Chapter.Markdown, "## Setup {#setup-2}\n"), "markdown = %q", second.Chapter.Markdown)
	_, err = BuildReadDocument(c, ReadDocumentIn{DocID: "guide", ChapterID: "nope", SectionID: ""}, 0)
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
	c, err := loader.Load(context.Background(), files)
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
	_, err = NewServer(c, Options{}).Connect(ctx, st, nil)
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

const sectionSample = `# Combined Documentation

---

<!-- Source: big/index.md -->

---
title: Big
---

# Big

## Install

Lead text.

### Prepare

The CIB must exist, see [Verify](#verify).

` + "```" + `
# zypper in demo
` + "```" + `

#### Repositories

Add the node repositories.

### Verify

` + "```" + `
# crm status
` + "```" + `
`

func TestReadDocumentSection(t *testing.T) {
	c := markdown.BuildDocuments(markdown.SplitSources([]byte(sectionSample+"\n---\n\n<!-- Source: big/ha-glossary.md -->\n\n# Glossary\n\nCIB (cluster information base)\n\nThe configuration.\n\nnode\n\nA cluster member.\n"), "combined.md"))
	toc, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "", SectionID: ""}, 0)
	require.NoError(t, err)
	require.Len(t, toc.TOC, 1)
	install := toc.TOC[0]
	assert.Positive(t, install.Chars)
	var ids []string
	for _, o := range install.Outline {
		ids = append(ids, o.ID)
		assert.Positive(t, o.Chars, o.ID)
	}
	assert.Equal(t, []string{"prepare", "verify"}, ids, "top level sections only")

	out, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", SectionID: "prepare"}, 0)
	require.NoError(t, err)
	ch := out.Chapter
	require.NotNil(t, ch)
	assert.Equal(t, "prepare", ch.SectionID)
	assert.Equal(t, "Prepare", ch.Section)
	assert.True(t, strings.HasPrefix(ch.Markdown, "### Prepare {#prepare}\n"), "markdown = %q", ch.Markdown)
	assert.Contains(t, ch.Markdown, "#### Repositories", "subsections are included")
	assert.NotContains(t, ch.Markdown, "Verify {#verify}", "following section is excluded")
	assert.NotContains(t, ch.Markdown, "Lead text")
	assert.Equal(t, []string{"zypper in demo"}, ch.Commands)
	require.Len(t, ch.Outline, 1)
	assert.Equal(t, "repositories", ch.Outline[0].ID)
	var terms []string
	for _, g := range ch.Glossary {
		terms = append(terms, g.Term)
	}
	assert.ElementsMatch(t, []string{"CIB (cluster information base)", "node"}, terms)
	require.Len(t, ch.References, 1)
	assert.Equal(t, "verify", ch.References[0].Linkend)

	verify, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", SectionID: "verify"}, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"crm status"}, verify.Chapter.Commands)
	assert.Empty(t, verify.Chapter.References)
	assert.Empty(t, verify.Chapter.Glossary)

	whole, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", SectionID: "install"}, 0)
	require.NoError(t, err)
	assert.Contains(t, whole.Chapter.Markdown, "Lead text", "the chapter id as section returns the whole chapter")

	_, err = BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", SectionID: "nope"}, 0)
	assert.Error(t, err, "unknown section")
	_, err = BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "", SectionID: "prepare"}, 0)
	assert.Error(t, err, "section without chapter")
}

func TestReadDocumentPaging(t *testing.T) {
	c := markdown.BuildDocuments(markdown.SplitSources([]byte(sectionSample), "combined.md"))
	full, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install"}, 0)
	require.NoError(t, err)
	whole := full.Chapter.Markdown
	assert.False(t, full.Chapter.Truncated)
	assert.Equal(t, chars(whole), full.Chapter.TotalChars)

	// read the chapter page by page with a server limit
	var joined strings.Builder
	var cmds []string
	pages := 0
	for offset := 0; ; pages++ {
		out, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", Offset: offset}, 60)
		require.NoError(t, err)
		ch := out.Chapter
		assert.Equal(t, offset, ch.Offset)
		assert.Equal(t, chars(whole), ch.TotalChars)
		assert.Equal(t, ch.NextOffset > 0, ch.Truncated)
		joined.WriteString(string([]rune(whole)[offset:max(ch.NextOffset, offset)]))
		cmds = append(cmds, ch.Commands...)
		if !ch.Truncated {
			joined.WriteString(string([]rune(whole)[offset:]))
			break
		}
		offset = ch.NextOffset
	}
	assert.Greater(t, pages, 1)
	assert.Equal(t, whole, joined.String())
	assert.ElementsMatch(t, []string{"zypper in demo", "crm status"}, cmds, "every command on exactly one page")

	// max_chars lowers the server limit but cannot raise it
	low, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", MaxChars: 60}, 0)
	require.NoError(t, err)
	assert.True(t, low.Chapter.Truncated)
	high, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", MaxChars: 100000}, 60)
	require.NoError(t, err)
	assert.True(t, high.Chapter.Truncated)

	// sections are paged as well
	sec, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", SectionID: "prepare"}, 40)
	require.NoError(t, err)
	assert.True(t, sec.Chapter.Truncated)
	assert.Equal(t, "prepare", sec.Chapter.SectionID)

	_, err = BuildReadDocument(c, ReadDocumentIn{DocID: "big", ChapterID: "install", Offset: 100000}, 60)
	assert.Error(t, err, "offset beyond the text")

	// a table of contents larger than the limit loses its outlines
	toc, err := BuildReadDocument(c, ReadDocumentIn{DocID: "big"}, 20)
	require.NoError(t, err)
	assert.Empty(t, toc.TOC[0].Outline)
	toc, err = BuildReadDocument(c, ReadDocumentIn{DocID: "big"}, 100000)
	require.NoError(t, err)
	assert.NotEmpty(t, toc.TOC[0].Outline)
}

func TestChapterResourceTruncated(t *testing.T) {
	c := markdown.BuildDocuments(markdown.SplitSources([]byte(sectionSample), "combined.md"))
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	_, err := NewServer(c, Options{MaxChars: 60}).Connect(ctx, st, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "docs://big/install"})
	require.NoError(t, err)
	text := res.Contents[0].Text
	assert.Contains(t, text, "Truncated after")
	assert.Contains(t, text, `chapter_id "install"`)

	tool, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "read_document", Arguments: map[string]any{"doc_id": "big", "chapter_id": "install"}})
	require.NoError(t, err)
	require.False(t, tool.IsError)
	data, _ := json.Marshal(tool.StructuredContent)
	assert.Contains(t, string(data), `"truncated":true`)
	assert.Contains(t, string(data), `"next_offset":`)
}
