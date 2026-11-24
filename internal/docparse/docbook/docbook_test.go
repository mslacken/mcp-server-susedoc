package docbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/markdown"
)

// writeFiles creates the files (name -> content) below a new temp dir.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	return root
}

const doctype = `<!DOCTYPE topic [
  <!ENTITY % entities SYSTEM "../common/generic-entities.ent">
  %entities;
]>
`

var entityFiles = map[string]string{
	"common/generic-entities.ent": `<!-- <!ENTITY commented "no"> -->
<!ENTITY % product-entities SYSTEM "product-entities.ent">
%product-entities;
<!ENTITY suse "SUSE">
<!ENTITY sles "&suse; Linux Enterprise Server">
<!ENTITY sles4sap '&sles; for SAP applications'>
<!ENTITY prompt.sudo "<prompt xmlns='http://docbook.org/ns/docbook'>&gt; </prompt><command xmlns='http://docbook.org/ns/docbook'>sudo</command> ">
<!ENTITY prompt.root "<prompt role='root' xmlns='http://docbook.org/ns/docbook'># </prompt>">
<!ENTITY loop "a &loop; b">
<!ENTITY nbsp "&#x000A0;">
`,
	"common/product-entities.ent": `<!ENTITY productname "<phrase xmlns='http://docbook.org/ns/docbook'><phrase os='sles'>&sles;</phrase><phrase os='sles4sap'>&sles4sap;</phrase></phrase>">
<!ENTITY productnumber "16.1">
<!ENTITY productnumber "17.0">
`,
}

func TestEntityExpansion(t *testing.T) {
	root := writeFiles(t, entityFiles)
	l := newEntityLoader()
	subset := `<!ENTITY sles "Local Server"> <!ENTITY % entities SYSTEM "../common/generic-entities.ent"> %entities;`
	tab := l.table(filepath.Join(root, "articles"), subset)
	for in, want := range map[string]string{
		"&productnumber;":            "16.1",
		"&sles;":                     "Local Server", // first declaration wins
		"&sles4sap;":                 "Local Server for SAP applications",
		"&suse;":                     "SUSE",
		"&prompt.sudo;zypper":        "<prompt xmlns='http://docbook.org/ns/docbook'>&gt; </prompt><command xmlns='http://docbook.org/ns/docbook'>sudo</command> zypper",
		"a &lt; b &amp; &#x41;":      "a &lt; b &amp; &#x41;",
		"&undefined;":                "&amp;undefined;",
		"&commented;":                "&amp;commented;",
		"<!-- &suse; -->&suse;":      "<!-- &suse; -->SUSE",
		"<![CDATA[&suse;]]>":         "<![CDATA[&suse;]]>",
		"&loop;":                     "a &amp;loop; b",
		"x&nbsp;y":                   "x&#x000A0;y",
		"<para os='a'>&suse;</para>": "<para os='a'>SUSE</para>",
	} {
		assert.Equal(t, want, tab.expand(in), "expand(%q)", in)
	}
	assert.Contains(t, tab.expand("&productname;"), "<phrase os='sles'>Local Server</phrase>")
	assert.Same(t, tab, l.table(filepath.Join(root, "articles"), subset), "table not cached")
}

func TestSplitDoctype(t *testing.T) {
	doc := `<?xml version="1.0"?>
<!-- comment ] > -->
<!DOCTYPE topic [
  <!-- a ] in a comment -->
  <!ENTITY x "va]l>ue">
]>
<topic/>`
	body, subset := splitDoctype(doc)
	assert.NotContains(t, body, "DOCTYPE")
	assert.Contains(t, body, "<topic/>")
	assert.Contains(t, body, "<!-- comment ] > -->")
	assert.Contains(t, subset, `<!ENTITY x "va]l>ue">`)
}

func TestProfile(t *testing.T) {
	root, err := decodeXML(`<topic><para os="sles;sles4sap">a</para><para os="sleha">b</para>` +
		`<para arch="x86-64">c</para><para arch="zseries">d</para><para condition="beta">e</para><para>f</para></topic>`)
	require.NoError(t, err)
	for _, tc := range []struct {
		p    profile
		want string
	}{
		{profile{os: profileValues("sles"), arch: profileValues("x86_64")}, "acef"},
		{profile{os: profileValues("sleha"), condition: profileValues("beta;suse-product")}, "bcdef"},
		{profile{}, "abcdef"},
	} {
		assert.Equal(t, tc.want, strings.ReplaceAll(tc.p.apply(root).flatText(), " ", ""), "%+v", tc.p)
	}
	assert.Equal(t, []string{"x86_64", "aarch64", "power"}, profileValues(" X86-64; arm ;;power"))
}

// docbookFixture is a small doc-modular like tree: two DC files publishing
// the same assembly for different products, one DC file with a missing
// assembly, a task with a procedure, an xincluded snippet, a container module
// and a glossary.
func docbookFixture(t *testing.T) string {
	files := map[string]string{
		"DC-SLES-demo": "# comment\nMAIN=\"demo.asm.xml\"\nSRC_DIR=\"articles\"\nIMG_SRC_DIR=\"images\"\nPROFOS=\"sles\"\nPROFCONDITION=\"suse-product;16.0\"\nSTRUCTID=\"demo-b\"\n#PROFOS=\"sleha\"\n",
		"DC-SAP-demo":  "MAIN='demo.asm.xml'\nSRC_DIR=articles\nPROFOS=\"sles4sap\"\nSTRUCTID=\"demo-b\"\n",
		"DC-SLES-gone": "MAIN=\"missing.asm.xml\"\nSRC_DIR=\"articles\"\n",
		"_DC-disabled": "MAIN=\"demo.asm.xml\"\nSRC_DIR=\"articles\"\n",
		"README.md":    "# Not a document\n",
		"articles/demo.asm.xml": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE assembly [
  <!ENTITY % entities SYSTEM "../common/generic-entities.ent">
  %entities;
]>
<assembly version="5.2" xmlns="http://docbook.org/ns/docbook" xmlns:xlink="http://www.w3.org/1999/xlink">
  <resources>
    <resource href="../tasks/install.xml" xml:id="_install"/>
    <resource href="../concepts/about.xml" xml:id="_about"/>
    <resource href="../references/glossary.xml" xml:id="_glossary"/>
    <resource href="../common/legal.xml" xml:id="_legal"/>
    <resource href="../tasks/does-not-exist.xml" xml:id="_unused"/>
    <!-- <resource href="../tasks/commented.xml" xml:id="_install"/> -->
  </resources>
  <structure xml:id="demo-a" renderas="article">
    <merge><title>Wrong structure</title></merge>
    <module resourceref="_about"/>
  </structure>
  <structure xml:id="demo-b" renderas="article">
    <merge>
      <title>Demo on &productname;</title>
      <subtitle>A subtitle</subtitle>
      <revhistory><revision><date>2026-01-02</date><revdescription><para>Initial
        version</para></revdescription></revision></revhistory>
      <meta name="maintainer" content="tux@example.com"/>
      <meta name="series">Products &amp; Solutions</meta>
      <meta name="task"><phrase>Installation</phrase><phrase>Administration</phrase></meta>
      <meta name="architecture"><phrase os="sles">x86</phrase><phrase os="sles4sap">ppc</phrase></meta>
      <meta name="productname"><productname version="&productnumber;" os="sles;sles4sap">&productname;</productname></meta>
      <meta name="description">Meta description</meta>
      <meta name="someone@example.com">malformed</meta>
      <meta name="updated" content="2026-02-03"/>
      <dm:docmanager xmlns:dm="urn:x-suse:ns:docmanager">
        <dm:bugtracker><dm:url>https://bugzilla.example.com</dm:url><dm:component>Docs</dm:component>
          <dm:product>Demo 16.1</dm:product><dm:assignee>tux@example.com</dm:assignee></dm:bugtracker>
        <dm:translation>yes</dm:translation>
      </dm:docmanager>
      <abstract><para>Lead of the <emphasis>demo</emphasis>.</para><para><emphasis role="bold">Prerequisites</emphasis></para><itemizedlist><listitem><para>none</para></listitem></itemizedlist></abstract>
    </merge>
    <module resourceref="_install" renderas="section">
      <merge><title>Install it</title></merge>
    </module>
    <module xml:id="more" renderas="section">
      <merge><title>More</title><abstract><para>Container abstract.</para></abstract></merge>
      <module resourceref="_about"/>
    </module>
    <module resourceref="_legal"/>
    <module resourceref="_glossary" renderas="glossary"/>
  </structure>
</assembly>
`,
		"tasks/install.xml": `<?xml version="1.0" encoding="UTF-8"?>
` + doctype + `<!-- refers to legacy doc: https://github.com/SUSE/doc-sle/blob/main/xml/install.xml -->
<topic xml:id="install" role="task" xmlns="http://docbook.org/ns/docbook" xmlns:xi="http://www.w3.org/2001/XInclude" xmlns:xlink="http://www.w3.org/1999/xlink">
  <info>
    <title>Installing</title>
    <meta name="maintainer" content="geeko@example.com"/>
    <abstract><para>How to install the <command>demo</command>.</para></abstract>
  </info>
  <para os="sles">Only on SLES.</para>
  <para os="sles4sap">Only on SAP.</para>
  <para>See <xref linkend="about-sec"/>, <xref linkend="dangling"/> and <link xlink:href="https://example.com/docs">the docs</link>.<remark>editorial</remark></para>
  <procedure xml:id="proc-install">
    <title>Installing the package</title>
    <step><para>Install the package:</para>
      <screen>&prompt.sudo;<command>zypper in <replaceable>PKG</replaceable></command><co xml:id="co-1"/></screen>
      <substeps><step><para>Check it.</para></step></substeps>
    </step>
    <xi:include href="../snippets/reboot-step.xml"/>
  </procedure>
  <note><title>Be careful</title><para>A note with <guimenu>Menu</guimenu>.</para></note>
  <variablelist><varlistentry><term><option>--force</option></term><listitem><para>Force it.</para></listitem></varlistentry></variablelist>
  <figure><title>Overview</title><mediaobject><imageobject><imagedata fileref="overview.png"/></imageobject></mediaobject></figure>
  <table><title>Options</title><tgroup cols="2"><thead><row><entry>Name</entry><entry>Meaning</entry></row></thead>
    <tbody><row><entry><literal>a|b</literal></entry><entry><para>One</para><para>Two</para></entry></row></tbody></tgroup></table>
  <section xml:id="install-verify"><title>Verifying</title>
    <screen language="bash">&prompt.root;rpm -q demo</screen>
  </section>
</topic>
`,
		"snippets/reboot-step.xml": `<!-- include like this: <xi:include href="../snippets/reboot-step.xml"/> -->
<!DOCTYPE step [
  <!ENTITY % entities SYSTEM "../common/generic-entities.ent">
  %entities;
]>
<step xmlns="http://docbook.org/ns/docbook"><para>Reboot:</para><screen>&prompt.sudo;reboot now</screen></step>
`,
		"concepts/about.xml": `<?xml version="1.0" encoding="UTF-8"?>
` + doctype + `<topic xml:id="about" role="concept" xmlns="http://docbook.org/ns/docbook">
  <title>About the demo</title>
  <info><abstract><para/></abstract></info>
  <para>The CIB of &productname;.</para>
  <section xml:id="about-sec"><title>Details</title><para>Details here.</para></section>
</topic>
`,
		"references/glossary.xml": `<?xml version="1.0" encoding="UTF-8"?>
` + doctype + `<topic xml:id="glossary" role="reference" xmlns="http://docbook.org/ns/docbook">
  <info><title>Glossary</title></info>
  <glossdiv><title>General</title>
  <glossentry xml:id="gloss-cib"><glossterm>CIB (cluster information base)</glossterm>
    <glossdef><para>The cluster configuration, see <xref linkend="gloss-node"/>.</para></glossdef></glossentry>
  </glossdiv>
  <glossentry xml:id="gloss-node"><glossterm>node</glossterm><glossdef><para>A <emphasis>member</emphasis>.</para></glossdef></glossentry>
</topic>
`,
		"common/legal.xml": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE topic [
  <!ENTITY % entities SYSTEM "generic-entities.ent">
  %entities;
]>
<topic xml:id="legal-disclaimer" xmlns="http://docbook.org/ns/docbook">
  <info><title>Legal Notice</title></info>
  <para>Permission is granted to copy, distribute and/or modify this document under the terms of the
    GNU Free Documentation License, Version 1.2 or (at your option) version 1.3; with the Invariant
    Section being this copyright notice and license.</para>
</topic>
`,
	}
	for k, v := range entityFiles {
		files[k] = v
	}
	return writeFiles(t, files)
}

// loadCorpus reads the DC files names of root, or all of them if none are
// given, through one shared Loader. As in the loader package, unreadable DC
// files are skipped and it is an error if none is left.
func loadCorpus(root string, names ...string) (*docparse.Corpus, error) {
	if len(names) == 0 {
		names = DCFiles(root)
	}
	l := NewLoader(root)
	c := docparse.NewCorpus()
	for _, name := range names {
		p, err := l.Read(name)
		if err != nil {
			warnf("skipping %s: %v", name, err)
			continue
		}
		c.Add(p)
	}
	if len(c.Documents) == 0 {
		return nil, fmt.Errorf("no usable DC files in %s", root)
	}
	c.Finish()
	return c, nil
}

// chapExtra returns the DocBook extras of ch, failing the test if it has none.
func chapExtra(t *testing.T, ch *docparse.Chapter) *ChapterExtra {
	t.Helper()
	e, ok := ch.Extra.(*ChapterExtra)
	require.True(t, ok, "chapter %q extra = %T", ch.ID, ch.Extra)
	return e
}

// searchDocs returns the document ids of the search hits.
func searchDocs(hits []docparse.SearchHit) []string {
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.DocID)
	}
	return ids
}

// commandDocs returns the document ids of the command hits.
func commandDocs(hits []docparse.CommandHit) []string {
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.DocID)
	}
	return ids
}

func TestDocBookCorpus(t *testing.T) {
	c, err := loadCorpus(docbookFixture(t))
	require.NoError(t, err)
	var ids []string
	for _, d := range c.Documents {
		ids = append(ids, d.ID)
	}
	require.Equal(t, []string{"sap-demo", "sles-demo"}, ids)

	sap, sles := c.Document("sap-demo"), c.Document("sles-demo")
	assert.Equal(t, "Demo on SUSE Linux Enterprise Server", sles.Title)
	assert.Equal(t, "Demo on SUSE Linux Enterprise Server for SAP applications", sap.Title)
	assert.Equal(t, "Lead of the demo.", sles.Summary)
	assert.Equal(t, docparse.Distro{Name: "SUSE Linux Enterprise Server", Versions: []string{"16.1", "16.0"}}, sles.Distro)
	assert.Equal(t, "GFDL-1.2-invariants-only OR GFDL-1.3-invariants-only", sles.License)
	require.IsType(t, &DocExtra{}, sles.Extras)
	require.IsType(t, &DocExtra{}, sap.Extras)
	slesExtra, sapExtra := sles.Extras.(*DocExtra), sap.Extras.(*DocExtra)
	assert.Equal(t, DocSource{DC: "DC-SLES-demo", Assembly: "articles/demo.asm.xml", Structure: "demo-b"}, *slesExtra.Source)
	assert.Equal(t, DocMeta{
		Maintainer: "tux@example.com", Series: "Products & Solutions", Tasks: []string{"Installation", "Administration"},
		Architectures: []string{"x86"}, Updated: "2026-02-03", Description: "Meta description",
	}, *slesExtra.Meta)
	if assert.NotEmpty(t, sapExtra.Meta.Architectures) {
		assert.Equal(t, "ppc", sapExtra.Meta.Architectures[0])
	}
	assert.Equal(t, "A subtitle", slesExtra.Subtitle)
	assert.Equal(t, "yes", slesExtra.Translation)
	assert.Equal(t, "Docs", slesExtra.Bugtracker.Component)
	assert.Equal(t, []Revision{{Date: "2026-01-02", Description: "Initial version"}}, slesExtra.Revisions)

	// legal notice and glossary are not chapters
	require.Len(t, sles.Chapters, 2)
	ch := sles.Chapters[0]
	chEx := chapExtra(t, &ch)
	assert.Equal(t, "install", ch.ID)
	assert.Equal(t, "1", chEx.Number)
	assert.Equal(t, "Install it", ch.Title)
	assert.Equal(t, "tasks/install.xml", ch.File)
	assert.Equal(t, "task", chEx.TopicType)
	assert.Equal(t, "geeko@example.com", chEx.Maintainer)
	assert.Equal(t, "How to install the demo.", ch.Summary)
	assert.Equal(t, []string{"sudo zypper in PKG", "sudo reboot now", "rpm -q demo"}, ch.Commands)
	assert.Equal(t, []string{"snippets/reboot-step.xml"}, chEx.Includes)
	assert.Equal(t, []string{"https://github.com/SUSE/doc-sle/blob/main/xml/install.xml"}, chEx.LegacySources)
	assert.Equal(t, []string{"about-sec", "dangling"}, ch.Xrefs)
	// Cross references resolve to their navigable target: "about-sec" is a
	// section of the "more" chapter in both documents (the own document is
	// preferred), "dangling" resolves nowhere and is reported as a dead link.
	assert.Equal(t, []docparse.XrefTarget{
		{Linkend: "about-sec", Text: "Details", Resolved: true, DocID: "sles-demo", ChapterID: "more", SectionID: "about-sec", Title: "Details", Scope: "section"},
		{Linkend: "dangling", Resolved: false},
	}, c.XrefTargets("sles-demo", &ch))
	assert.Equal(t, []Link{{URL: "https://example.com/docs", Text: "the docs"}}, chEx.Links)
	assert.Equal(t, []Image{{File: "images/overview.png", Alt: "Overview"}}, chEx.Images)
	assert.Equal(t, []Procedure{{ID: "proc-install", Number: "1", Title: "Installing the package", Steps: 2}}, chEx.Procedures)
	assert.Equal(t, []Admonition{{Type: "note", Title: "Be careful"}}, chEx.Admonitions)
	assert.Equal(t, []docparse.Section{{ID: "install-verify", Number: "1.1", Title: "Verifying", Level: 1}}, ch.Sections)

	md := c.Text(&ch)
	for _, want := range []string{
		"## 1 Install it {#install}\n\nHow to install the `demo`.\n\nOnly on SLES.\n\nSee [Details](#about-sec), [dangling](#dangling) and [the docs](https://example.com/docs).\n",
		"**Procedure 1: Installing the package**\n\n1. Install the package:\n\n    ```console\n    > sudo zypper in PKG <1>\n    ```\n\n    1. Check it.\n\n2. Reboot:\n",
		"> **Note:** Be careful\n>\n> A note with **Menu**.\n",
		"- **`--force`**\n\n    Force it.\n",
		"**Figure 1: Overview**\n\n![Overview](images/overview.png)\n",
		"**Table 1: Options**\n\n| Name | Meaning |\n| --- | --- |\n| `a\\|b` | One<br>Two |\n",
		"### 1.1 Verifying {#install-verify}\n\n```bash\n# rpm -q demo\n```",
	} {
		assert.Contains(t, md, want)
	}
	// profiled or editorial text
	assert.NotContains(t, md, "Only on SAP")
	assert.NotContains(t, md, "editorial")
	assert.Contains(t, c.Text(&sap.Chapters[0]), "Only on SAP.", "SAP variant lacks its profiled paragraph")

	more := sles.Chapters[1]
	moreEx := chapExtra(t, &more)
	assert.Equal(t, "more", more.ID)
	assert.Equal(t, "2", moreEx.Number)
	assert.Equal(t, "container", moreEx.TopicType)
	assert.Equal(t, "Container abstract.", more.Summary)
	assert.Equal(t, []docparse.Section{{ID: "about", Number: "2.1", Title: "About the demo", Level: 1, File: "concepts/about.xml"}, {ID: "about-sec", Number: "2.1.1", Title: "Details", Level: 2}}, more.Sections)
	assert.True(t, strings.HasPrefix(c.Text(&more), "## 2 More {#more}\n\nContainer abstract.\n\n### 2.1 About the demo {#about}\n\nThe CIB of SUSE Linux Enterprise Server.\n\n#### 2.1.1 Details {#about-sec}"),
		"container markdown = %q", c.Text(&more))
	if terms := c.GlossaryForChapter(&more); assert.Len(t, terms, 1) {
		assert.Equal(t, "CIB (cluster information base)", terms[0].Term)
	}

	// shared glossary, deduplicated across the two documents
	if got := c.GlossaryEntries(); assert.Len(t, got, 2) {
		assert.Equal(t, "CIB (cluster information base)", got[0].Term)
		assert.Equal(t, "The cluster configuration, see node.", got[0].WhatIs)
		assert.Equal(t, "node", got[1].Term)
		assert.Equal(t, "A member.", got[1].WhatIs)
	}
	assert.Len(t, c.FindCommand("zypper", nil), 2)
	assert.Equal(t, []string{"sles-demo"}, commandDocs(c.FindCommand("zypper", []string{"sles-demo"})))
	assert.Equal(t, []string{"sap-demo"}, commandDocs(c.FindCommand("zypper", []string{"sap-demo"})))
	assert.Empty(t, c.FindCommand("zypper", []string{"no-such-doc"}))
	assert.Equal(t, []string{"sap-demo"}, searchDocs(c.Search("reboot", "for SAP", "", "", 0, 1, false, nil)), "search")
	assert.Equal(t, []string{"sap-demo"}, searchDocs(c.Search("rebo.", "for SAP", "", "", 0, 0, true, nil)), "regex search")
	assert.Equal(t, []string{"sles-demo"}, searchDocs(c.Search("zypper", "", "", "", 0, 0, false, []string{"sles-demo"})), "search (sles-demo)")
	assert.Empty(t, c.Search("zypper", "", "", "", 0, 0, false, []string{"no-such-doc"}), "search (no-such-doc)")
	// scope: "reboot" and "zypper" are body-only, "install" is title-only,
	// "container" appears only in a summary
	assert.Empty(t, c.Search("reboot", "for SAP", "", "title", 0, 1, false, nil), "title search for body word")
	assert.Empty(t, c.Search("zypper", "for SAP", "", "summary", 0, 1, false, nil), "summary search for body word")
	assert.Empty(t, c.Search("rebo.", "for SAP", "", "body", 0, 0, true, nil), "regex search with an unknown scope")
	if hits := c.Search("install", "for SAP", "", "title", 0, 0, false, nil); assert.Len(t, hits, 1, "title search") {
		assert.Equal(t, "sap-demo", hits[0].DocID)
		assert.Equal(t, "Install it", hits[0].Snippet)
	}
	assert.Empty(t, c.Search("reboot", "for SAP", "", "title", 0, 0, true, nil), "regex title search for body pattern")
	if hits := c.Search("container", "for SAP", "", "summary", 0, 0, false, nil); assert.Len(t, hits, 1, "summary search") {
		assert.Equal(t, "Container abstract.", hits[0].Snippet)
	}
}

func TestSingleDCFile(t *testing.T) {
	root := docbookFixture(t)
	c, err := loadCorpus(root, "DC-SLES-demo")
	require.NoError(t, err)
	if assert.Len(t, c.Documents, 1) {
		assert.Equal(t, "sles-demo", c.Documents[0].ID)
	}
	_, err = loadCorpus(root, "DC-SLES-gone")
	assert.Error(t, err, "expected error for a DC file without assembly")
}

func TestStructureFallback(t *testing.T) {
	asm, err := decodeXML(`<assembly><structure xml:id="a"/><structure xml:id="b"/></assembly>`)
	require.NoError(t, err)
	for id, want := range map[string]string{"": "a", "b": "b", "nope": "a"} {
		assert.Equal(t, want, selectStructure(asm, id).attr["id"], "selectStructure(%q)", id)
	}
}

func TestBookNumbering(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"DC-set": "MAIN=\"set.xml\"\n",
		"xml/set.xml": `<set xmlns="http://docbook.org/ns/docbook" xmlns:xi="http://www.w3.org/2001/XInclude" xml:id="set">
  <info><title>All</title></info>
  <book xml:id="b1"><info><title>Book one</title></info>
    <preface xml:id="pre"><title>About</title><section xml:id="pre-s"><title>Conventions</title><para>x</para></section></preface>
    <part xml:id="p1"><title>Part</title>
      <chapter xml:id="c1"><title>First</title><para>Intro.</para>
        <procedure><title>Doing</title><step><para>Do.</para></step></procedure>
        <procedure><step><para>Untitled.</para></step></procedure>
        <section xml:id="c1-s1"><title>One</title>
          <figure><title>Pic</title><mediaobject><imageobject><imagedata fileref="a.png"/></imageobject></mediaobject></figure>
          <section xml:id="c1-s1-1"><title>Deep</title><para>d</para></section>
        </section>
        <section xml:id="c1-s2"><title>Two</title><informaltable><tgroup cols="1"><tbody><row><entry>e</entry></row></tbody></tgroup></informaltable></section>
      </chapter>
      <xi:include href="chap2.xml"/>
    </part>
    <appendix xml:id="app"><title>Extra</title><section xml:id="app-s"><title>More</title><para>m</para></section></appendix>
    <appendix role="legal"><title>GNU licenses</title><para>GNU Free Documentation License Version 1.2</para></appendix>
  </book>
  <book xml:id="b2"><info><title>Book two</title></info>
    <chapter xml:id="c3"><title>Again first</title><para>a</para></chapter>
  </book>
</set>
`,
		"xml/chap2.xml": `<chapter xmlns="http://docbook.org/ns/docbook" xml:id="c2"><title>Second</title>
  <table><title>T</title><tgroup cols="1"><tbody><row><entry>e</entry></row></tbody></tgroup></table></chapter>
`,
	})
	c, err := loadCorpus(root)
	require.NoError(t, err)
	doc := c.Documents[0]
	var got []string
	for i := range doc.Chapters {
		ch := &doc.Chapters[i]
		got = append(got, chapExtra(t, ch).Number+" "+ch.Title)
	}
	require.Equal(t, []string{" About", "1 First", "2 Second", "A Extra", "1 Again first"}, got)
	pre, first, app := doc.Chapters[0], doc.Chapters[1], doc.Chapters[3]
	preEx, firstEx := chapExtra(t, &pre), chapExtra(t, &first)
	assert.Equal(t, "1", pre.Sections[0].Number)
	assert.Equal(t, "preface", preEx.RenderAs)
	assert.Equal(t, []docparse.Section{{ID: "c1-s1", Number: "1.1", Title: "One", Level: 1}, {ID: "c1-s1-1", Number: "1.1.1", Title: "Deep", Level: 2}, {ID: "c1-s2", Number: "1.2", Title: "Two", Level: 1}}, first.Sections)
	assert.Equal(t, "1.1", firstEx.Procedures[0].Number)
	assert.Empty(t, firstEx.Procedures[1].Number)
	assert.Equal(t, "A.1", app.Sections[0].Number)
	md := c.Text(&first)
	for _, want := range []string{"## 1 First {#c1}", "**Procedure 1.1: Doing**", "### 1.1 One {#c1-s1}", "**Figure 1.1: Pic**", "#### 1.1.1 Deep {#c1-s1-1}"} {
		assert.Contains(t, md, want)
	}
	assert.Contains(t, c.Text(&doc.Chapters[2]), "**Table 2.1: T**")
	assert.NotEmpty(t, doc.License, "legal appendix not taken as license")
}

// render renders the DocBook fragment src as the blocks of a chapter.
func render(t *testing.T, src string) string {
	t.Helper()
	n, err := decodeXML(src)
	require.NoError(t, err)
	r := newRenderer("")
	r.ch = &docparse.Chapter{}
	r.extra = &ChapterExtra{}
	return strings.Join(joinBlocks(r.blocks(n.kids, 2, 1)), "\n")
}

func TestCallouts(t *testing.T) {
	md := render(t, `<x><screen>&gt; <command>oscap</command> eval<co xml:id="co-a"/> \
  --profile p<co xml:id="co-b"/>
Type=forking <co xml:id="co-c"/>
PIDFile=FILE<xref linkend="co-a"/></screen>
<calloutlist><callout arearefs="co-a"><para>Evaluates.</para></callout><callout arearefs="co-b co-c"><para>Both.</para></callout></calloutlist>
<para>See <xref linkend="co-b"/>.</para></x>`)
	want := "```\n> oscap eval <1> \\\n  --profile p <2>\nType=forking <3>\nPIDFile=FILE <1>\n```\n\n- <1> Evaluates.\n\n- <2> <3> Both.\n\nSee <2>."
	assert.Equal(t, want, md)
	assert.Equal(t, []string{"oscap eval --profile p", "virsh vol-create-as POOL NAME 12G"},
		markdown.CommandsInText("> oscap eval <1> \\\n  --profile p <2>\n> virsh vol-create-as POOL <1>NAME <2> 12G"))
}

func TestTableSpans(t *testing.T) {
	md := render(t, `<x><informaltable><tgroup cols="3">
  <colspec colname="c1"/><colspec colname="c2"/><colspec colname="c3"/>
  <thead><row><entry>Group</entry><entry namest="c2" nameend="c3">Keys</entry></row></thead>
  <tbody>
    <row><entry morerows="2">Common</entry><entry>^J</entry><entry>engage</entry></row>
    <row><entry>^L</entry><entry>cancel</entry></row>
    <row><entry colname="c3">only last</entry></row>
    <row><entry namest="c1" nameend="c3">full width</entry></row>
    <row><entry>a</entry><entry>b</entry><entry>c</entry></row>
  </tbody></tgroup></informaltable>
<informaltable><tr><td rowspan="2">r</td><td colspan="2">wide</td></tr><tr><td>x</td><td>y</td></tr></informaltable></x>`)
	want := `| Group | Keys |  |
| --- | --- | --- |
| Common | ^J | engage |
| Common | ^L | cancel |
| Common |  | only last |
| full width |  |  |
| a | b | c |

| r | wide |  |
| --- | --- | --- |
| r | x | y |`
	assert.Equal(t, want, md)
}
