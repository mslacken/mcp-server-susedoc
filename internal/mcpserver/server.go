package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	rdebug "runtime/debug"
	"sort"
	"strings"

	"mcp-server-susedoc/internal/debug"
	"mcp-server-susedoc/internal/docparse"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var debugf = debug.New("tool")

type listDocumentsIn struct {
	Product string `json:"product,omitempty" jsonschema:"only documents whose product contains this text, e.g. 'Register Product'"`
	Version string `json:"version,omitempty" jsonschema:"only documents for this product version, e.g. '16.0'"`
}

type documentInfo struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Distro   docparse.Distro  `json:"distro"`
	Product  docparse.Product `json:"product,omitempty"`
	Chapters int              `json:"chapters"`
}

type ListDocumentsOut struct {
	Documents []documentInfo `json:"documents"`
}

type readDocumentIn struct {
	DocID     string `json:"doc_id" jsonschema:"document id as returned by list_documents or search_docs"`
	ChapterID string `json:"chapter_id,omitempty" jsonschema:"chapter id from the table of contents; if omitted only the table of contents is returned"`
}

type tocEntry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Commands int    `json:"commands"`
}

type chapterOut struct {
	ID         string                   `json:"id"`
	Title      string                   `json:"title"`
	File       string                   `json:"file,omitempty"`
	Markdown   string                   `json:"markdown"`
	Commands   []string                 `json:"commands,omitempty"`
	Glossary   []docparse.GlossaryEntry `json:"glossary,omitempty"`
	References []docparse.XrefTarget    `json:"references,omitempty"`
}

type ReadDocumentOut struct {
	documentInfo
	TOC     []tocEntry  `json:"toc,omitempty"`
	Chapter *chapterOut `json:"chapter,omitempty"`
}

type searchDocsIn struct {
	Query   string   `json:"query" jsonschema:"search words"`
	Product string   `json:"product,omitempty" jsonschema:"only documents whose product contains this text"`
	Version string   `json:"version,omitempty" jsonschema:"only documents for this product version"`
	Scope   string   `json:"scope,omitempty" jsonschema:"where to search within a chapter: 'all' (title, summary and body, default), 'title' or 'summary'"`
	Limit   int      `json:"limit,omitempty" jsonschema:"maximum number of hits, default 10"`
	Fuzzy   int      `json:"fuzzy,omitempty" jsonschema:"maximum edit distance for a word to still match, default 1 (0 disables fuzzy)"`
	Regex   bool     `json:"regex,omitempty" jsonschema:"treat query as a regular expression instead of words"`
	Docs    []string `json:"docs,omitempty" jsonschema:"only search in the documents with these ids, as returned by list_documents; empty searches the whole corpus"`
}

type SearchDocsOut struct {
	Hits []docparse.SearchHit `json:"hits"`
}

type findCommandIn struct {
	Pattern string   `json:"pattern" jsonschema:"words the command must contain, e.g. 'crm configure primitive'"`
	Docs    []string `json:"docs,omitempty" jsonschema:"only search in the documents with these ids, as returned by list_documents; empty searches the whole corpus"`
}

type FindCommandOut struct {
	Commands []docparse.CommandHit `json:"commands"`
}

type defineTermIn struct {
	Term string   `json:"term" jsonschema:"glossary term or abbreviation, e.g. 'CIB' or 'split brain'"`
	Docs []string `json:"docs,omitempty" jsonschema:"only look in documents with these ids, as returned by list_documents; empty searches all documents"`
}

type DefineTermOut struct {
	Entries []docparse.GlossaryEntry `json:"entries"`
}

type ListGlossaryOut struct {
	Entries []glossaryEntry `json:"entries"`
}

// glossaryEntry is a glossary term with its definition and the documents that
// use it.
type glossaryEntry struct {
	Term   string   `json:"term"`
	WhatIs string   `json:"whatis"`
	Docs   []string `json:"docs,omitempty"`
}

func info(d *docparse.Document) documentInfo {
	return documentInfo{ID: d.ID, Title: d.Title, Distro: d.Distro, Product: d.Product, Chapters: len(d.Chapters)}
}

func BuildReadDocument(c *docparse.Corpus, docID, chapterID string) (ReadDocumentOut, error) {
	doc := c.Document(docID)
	if doc == nil {
		return ReadDocumentOut{}, fmt.Errorf("unknown document %q, use list_documents to get the ids", docID)
	}
	out := ReadDocumentOut{documentInfo: info(doc)}
	if chapterID == "" {
		out.TOC = make([]tocEntry, 0, len(doc.Chapters))
		for _, ch := range doc.Chapters {
			out.TOC = append(out.TOC, tocEntry{ID: ch.ID, Title: ch.Title, Summary: ch.Summary, Commands: len(ch.Commands)})
		}
		return out, nil
	}
	ch := doc.Chapter(chapterID)
	if ch == nil {
		return ReadDocumentOut{}, fmt.Errorf("unknown chapter %q in document %q", chapterID, docID)
	}
	refs := nonNil(c.XrefTargets(doc.ID, ch))
	out.Chapter = &chapterOut{
		ID: ch.ID, Title: ch.Title, File: ch.File,
		Markdown: docparse.ServedText(c.Text(ch), ch, refs), Commands: nonNil(ch.Commands), Glossary: c.GlossaryForChapter(ch),
		References: refs,
	}
	return out, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// ListDocuments returns the documents matching product and version; an empty
// selector matches everything.
func ListDocuments(c *docparse.Corpus, product, version string) ListDocumentsOut {
	out := ListDocumentsOut{Documents: []documentInfo{}}
	for i := range c.Documents {
		if c.Documents[i].Matches(product, version) {
			out.Documents = append(out.Documents, info(&c.Documents[i]))
		}
	}
	return out
}

// SearchDocs ranks chapters for query, optionally filtered by product/version
// and restricted to the documents in docs. scope limits the search to a
// chapter's "title" or "summary" ("all" or empty searches everything); fuzzy
// is the maximum edit distance allowed for a word match; regex treats the
// query as a regular expression.
func SearchDocs(c *docparse.Corpus, query, product, version, scope string, limit, fuzzy int, regex bool, docs []string) SearchDocsOut {
	if limit <= 0 {
		limit = 10
	}
	return SearchDocsOut{Hits: nonNil(c.Search(query, product, version, scope, limit, fuzzy, regex, docs))}
}

// FindCommand returns the documented commands containing every word of pattern,
// optionally restricted to the documents in docs.
func FindCommand(c *docparse.Corpus, pattern string, docs []string) FindCommandOut {
	return FindCommandOut{Commands: nonNil(c.FindCommand(pattern, docs))}
}

// DefineTerm looks up term in the glossary, falling back to partial matches,
// optionally restricted to the documents in docs.
func DefineTerm(c *docparse.Corpus, term string, docs []string) DefineTermOut {
	return DefineTermOut{Entries: nonNil(c.DefineTerm(term, docs))}
}

// ListGlossary returns every glossary entry in term order, each with the ids of
// the documents whose chapters mention it.
func ListGlossary(c *docparse.Corpus) ListGlossaryOut {
	entries := make([]glossaryEntry, 0, len(c.Glossary))
	for _, e := range c.GlossaryEntries() {
		entries = append(entries, glossaryEntry{Term: e.Term, WhatIs: e.WhatIs, Docs: c.DocsUsing(e.Term)})
	}
	return ListGlossaryOut{Entries: entries}
}

// corpusSummary is the shape of the corpus overview endpoint.
type CorpusOverview struct {
	Documents int      `json:"documents"`
	Chapters  int      `json:"chapters"`
	Glossary  int      `json:"glossary"`
	Products  []string `json:"products"`
	Versions  []string `json:"versions"`
	Terms     int      `json:"terms"`
	Commands  int      `json:"commands"`
}

// CorpusSummary returns aggregate counts and the distinct product/version
// values, plus the number of glossary terms and documented commands.
func CorpusSummary(c *docparse.Corpus) CorpusOverview {
	s := CorpusOverview{
		Documents: len(c.Documents),
		Glossary:  len(c.Glossary),
		Terms:     len(c.Glossary),
	}
	products := map[string]bool{}
	versions := map[string]bool{}
	for i := range c.Documents {
		d := &c.Documents[i]
		if d.Distro.Name != "" {
			products[d.Distro.Name] = true
		}
		for _, v := range d.Distro.Versions {
			versions[v] = true
		}
		for j := range d.Chapters {
			s.Chapters++
			s.Commands += len(d.Chapters[j].Commands)
		}
	}
	s.Products = sortedKeys(products)
	s.Versions = sortedKeys(versions)
	return s
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// setEnum restricts a property to the given values. An empty list leaves the
// property unrestricted. The enum is derived from the corpus, so it only ever
// contains products and versions that actually occur in the documents.
func setEnum(s *jsonschema.Schema, prop string, values []string) {
	if p := s.Properties[prop]; p != nil && len(values) > 0 {
		p.Enum = make([]any, len(values))
		for i, v := range values {
			p.Enum[i] = v
		}
	}
}

// CreateListDocumentsSchema returns the input schema for list_documents with
// product and version restricted to the values present in the corpus, or
// unrestricted if c is nil.
func CreateListDocumentsSchema(c *docparse.Corpus) *jsonschema.Schema {
	schema, _ := jsonschema.For[listDocumentsIn](nil)
	if schema != nil && c != nil {
		ov := CorpusSummary(c)
		setEnum(schema, "product", ov.Products)
		setEnum(schema, "version", ov.Versions)
	}
	return schema
}

// CreateSearchDocsSchema returns the input schema for search_docs with product
// and version restricted to the values present in the corpus and scope to the
// supported values. Product and version are unrestricted if c is nil.
func CreateSearchDocsSchema(c *docparse.Corpus) *jsonschema.Schema {
	schema, _ := jsonschema.For[searchDocsIn](nil)
	if schema != nil {
		if c != nil {
			ov := CorpusSummary(c)
			setEnum(schema, "product", ov.Products)
			setEnum(schema, "version", ov.Versions)
		}
		setEnum(schema, "scope", []string{"all", "title", "summary"})
	}
	return schema
}

// Version is reported to MCP clients and by -version. Release builds set it
// with -ldflags "-X mcp-server-susedoc/internal/mcpserver.Version=v1.2.3";
// otherwise the module version from `go install ...@v1.2.3` is used.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if bi, ok := rdebug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}
}

// NewServer builds the MCP server exposing the corpus' documents.
func NewServer(c *docparse.Corpus) *mcp.Server {
	return NewLoadingServer(Ready(c))
}

// NewLoadingServer builds the MCP server exposing the documents of a corpus
// that may still be loading. Its tools are listed right away, without the
// product and version enums, and are replaced with ones restricted to the
// values in the corpus once it is loaded, which notifies the clients with
// notifications/tools/list_changed. Tool calls and resource reads wait for
// the corpus.
func NewLoadingServer(p *PendingCorpus) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "mcp-server-susedoc", Version: Version}, &mcp.ServerOptions{
		Instructions: "Product documentation. Use search_docs or list_documents to find a document, " +
			"read_document to get its table of contents or a chapter, find_command to find where a " +
			"command is documented and define_term for the glossary.",
	})
	addTools(srv, p)
	addResources(srv, p)
	if p.loaded() == nil {
		go func() {
			if _, err := p.Wait(context.Background(), nil); err == nil {
				debugf("corpus loaded, updating the tool schemas")
				addTools(srv, p)
			}
		}()
	}
	return srv
}

// addTools adds the tools to srv, replacing those already added. The input
// schemas list the products and versions of the corpus if it is loaded.
func addTools(srv *mcp.Server, p *PendingCorpus) {
	loaded := p.loaded()

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_documents",
		Description: "List the available documents (guides) with id, title, summary and product versions.",
		InputSchema: CreateListDocumentsSchema(loaded),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listDocumentsIn) (*mcp.CallToolResult, ListDocumentsOut, error) {
		debugf("list_documents product=%q version=%q", in.Product, in.Version)
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, ListDocumentsOut{}, err
		}
		return nil, ListDocuments(c, in.Product, in.Version), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "read_document",
		Description: "Return a document's metadata and table of contents. If chapter_id is given, return " +
			"that chapter's markdown, the commands it shows, the definitions of all glossary terms it mentions " +
			"and where the chapter's cross references point to (unresolved ones are reported as dead), " +
			"each resolved reference with the docs:// uri of the resource to read its target; " +
			"links in the markdown point to these uris and headings carry their {#id}, " +
			"without the table of contents.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in readDocumentIn) (*mcp.CallToolResult, ReadDocumentOut, error) {
		debugf("read_document doc_id=%q chapter_id=%q", in.DocID, in.ChapterID)
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, ReadDocumentOut{}, err
		}
		out, err := BuildReadDocument(c, in.DocID, in.ChapterID)
		debugf("read_document result: %d toc entries, chapter=%v err=%v", len(out.TOC), out.Chapter != nil, err)
		return nil, out, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_docs",
		Description: "Full text search over all chapters. Returns ranked hits with doc_id and chapter_id for read_document.",
		InputSchema: CreateSearchDocsSchema(loaded),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchDocsIn) (*mcp.CallToolResult, SearchDocsOut, error) {
		debugf("search_docs query=%q product=%q version=%q scope=%q limit=%d fuzzy=%d regex=%v docs=%q", in.Query, in.Product, in.Version, in.Scope, in.Limit, in.Fuzzy, in.Regex, in.Docs)
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, SearchDocsOut{}, err
		}
		out := SearchDocs(c, in.Query, in.Product, in.Version, in.Scope, in.Limit, in.Fuzzy, in.Regex, in.Docs)
		debugf("search_docs result: %d hits", len(out.Hits))
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_command",
		Description: "Find the documented shell commands containing all given words and the chapters showing them.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in findCommandIn) (*mcp.CallToolResult, FindCommandOut, error) {
		debugf("find_command pattern=%q docs=%q", in.Pattern, in.Docs)
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, FindCommandOut{}, err
		}
		out := FindCommand(c, in.Pattern, in.Docs)
		debugf("find_command result: %d commands", len(out.Commands))
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "define_term",
		Description: "Look up a term or abbreviation in the glossary.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in defineTermIn) (*mcp.CallToolResult, DefineTermOut, error) {
		debugf("define_term term=%q docs=%q", in.Term, in.Docs)
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, DefineTermOut{}, err
		}
		out := DefineTerm(c, in.Term, in.Docs)
		debugf("define_term result: %d entries", len(out.Entries))
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_glossary",
		Description: "List every glossary term and definition, with the documents that use each term.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, ListGlossaryOut, error) {
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, ListGlossaryOut{}, err
		}
		out := ListGlossary(c)
		debugf("list_glossary result: %d entries", len(out.Entries))
		return nil, out, nil
	})
}

func addResources(srv *mcp.Server, p *PendingCorpus) {
	jsonResult := func(uri string, v any) (*mcp.ReadResourceResult, error) {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(data)}}}, nil
	}

	srv.AddResource(&mcp.Resource{
		URI: "docs://index", Name: "index", MIMEType: "application/json",
		Description: "All documents with id, title, summary and product versions",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, err
		}
		docs := []documentInfo{}
		for i := range c.Documents {
			docs = append(docs, info(&c.Documents[i]))
		}
		return jsonResult(req.Params.URI, docs)
	})

	srv.AddResource(&mcp.Resource{
		URI: "docs://glossary", Name: "glossary", MIMEType: "application/json",
		Description: "The glossary shared by all documents",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, err
		}
		return jsonResult(req.Params.URI, c.Glossary)
	})

	// docs://{doc_id} and docs://{doc_id}/{chapter_id}, same content as read_document
	handler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		c, err := p.Wait(ctx, req)
		if err != nil {
			return nil, err
		}
		uri := req.Params.URI
		// the #section_id of a reference uri names a section, the whole chapter is returned
		path, _, _ := strings.Cut(strings.TrimPrefix(uri, "docs://"), "#")
		docID, chapterID, _ := strings.Cut(path, "/")
		out, err := BuildReadDocument(c, docID, chapterID)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if out.Chapter != nil {
			text := out.Chapter.Markdown + referencesMarkdown(out.Chapter.References)
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}}}, nil
		}
		return jsonResult(uri, out)
	}
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "docs://{doc_id}", Name: "document", MIMEType: "application/json",
		Description: "Metadata and table of contents of a document",
	}, handler)
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "docs://{doc_id}/{chapter_id}{#section_id}", Name: "chapter", MIMEType: "text/markdown",
		Description: "Markdown of a chapter, followed by a References list with the docs:// URI of every cross reference target",
	}, handler)
}

// referencesMarkdown lists the cross references of a chapter as a markdown
// section of links to their docs:// URIs; unresolved ones are marked as such.
func referencesMarkdown(refs []docparse.XrefTarget) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## References\n\n")
	for _, r := range refs {
		title := r.Title
		if title == "" {
			title = r.Linkend
		}
		if r.Resolved {
			fmt.Fprintf(&b, "- [%s](%s)\n", title, r.URI)
		} else {
			fmt.Fprintf(&b, "- %s (unresolved)\n", r.Linkend)
		}
	}
	return b.String()
}
