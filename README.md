# Present openSUSE/SUSE documentation to an LLM

Having an MCP server that reads openSUSE/SUSE documentation improves the
quality of answers to related questions about these distributions a lot.
This project delivers a server that reads

* DocBook (the DC files SUSE and openSUSE publish)
* markdown
* plain text

and presents every document through the MCP tools and resources described
below. Parsed documents can also be exported to a JSON format and read back,
so the server can run from prepackaged exports without the original sources.

## Data model

Every document follows the Go structs defined in
[./internal/docparse/document.go](./internal/docparse/document.go). Only the
exported, serialized fields are shown; format specific extras (like the
DocBook provenance) and internal search data are not part of the output.

```go
// Corpus is the set of all documents, as printed by -dump.
type Corpus struct {
    Documents []Document               `json:"documents" yaml:"documents"`
    Glossary  map[string]GlossaryEntry `json:"glossary" yaml:"glossary"` // key: lower case term
}

type Document struct {
    ID       string    `json:"id" yaml:"id"`
    Title    string    `json:"title" yaml:"title"`
    Summary  string    `json:"summary" yaml:"summary"`
    Distro   Distro    `json:"distro" yaml:"distro"`
    Product  Product   `json:"product" yaml:"product"`
    Chapters []Chapter `json:"chapters" yaml:"chapters"`
    License  string    `json:"license,omitempty" yaml:"license,omitempty"`
}

type Distro struct {
    Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
    Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`
}

type Product struct {
    Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
    Versions []string `json:"versions,omitempty" yaml:"versions,omitempty"`
}

type Chapter struct {
    ID       string    `json:"id" yaml:"id"`
    Title    string    `json:"chapter_title" yaml:"chapter_title"`
    Summary  string    `json:"summary,omitempty" yaml:"summary,omitempty"`
    File     string    `json:"file,omitempty" yaml:"file,omitempty"`
    Commands []string  `json:"commands,omitempty" yaml:"commands,omitempty"`
    Sections []Section `json:"sections,omitempty" yaml:"sections,omitempty"`
    Xrefs    []string  `json:"xrefs,omitempty" yaml:"xrefs,omitempty"`
}

type Section struct {
    ID     string `json:"id,omitempty" yaml:"id,omitempty"`
    Number string `json:"number,omitempty" yaml:"number,omitempty"`
    Title  string `json:"title" yaml:"title"`
    Level  int    `json:"level" yaml:"level"`
    File   string `json:"file,omitempty" yaml:"file,omitempty"`
}

type GlossaryEntry struct {
    Term   string `json:"term" yaml:"term"`
    WhatIs string `json:"whatis" yaml:"whatis"`
}
```

### Field reference

**Document**

| Field | Meaning |
|-------|---------|
| `id` | Unique document id, also used as file name of the export (`<id>.json`) |
| `title` | Title of the guide |
| `summary` | Abstract of the guide |
| `distro` | Distribution the document applies to, e.g. `SUSE Linux Enterprise Server` with versions `16.0`, `16.1` |
| `product` | Specific product the document applies to, e.g. `SLES for SAP Applications`, with its own versions |
| `chapters` | Top level sections of the document |
| `license` | License of the document, if known |

**Chapter**

| Field | Meaning |
|-------|---------|
| `id` | Chapter id, unique within the document |
| `chapter_title` | Chapter heading |
| `summary` | Lead paragraphs of the chapter |
| `file` | File the chapter was written in: the DocBook topic file, the markdown file (the original one for combined files) or the text file |
| `commands` | Shell commands shown in the chapter |
| `sections` | Headings below the chapter heading |
| `xrefs` | Linkends (target ids) of the chapter's cross references; only DocBook chapters have them |

**Section**

| Field | Meaning |
|-------|---------|
| `id` | Section id (e.g. the DocBook `xml:id`) |
| `number` | Section number as published, e.g. `19.6.9`; empty for unnumbered sections |
| `title` | Section heading |
| `level` | Depth below the chapter, `1` is a direct subsection |
| `file` | Topic file the section was taken from (nested modules) |

`read_document` resolves a chapter's `xrefs` against the whole corpus and
returns them as `references`, a list of `XrefTarget`:

```go
type XrefTarget struct {
    Linkend   string `json:"linkend"`
    Text      string `json:"text,omitempty"`
    Resolved  bool   `json:"resolved"`
    DocID     string `json:"doc_id,omitempty"`
    ChapterID string `json:"chapter_id,omitempty"`
    SectionID string `json:"section_id,omitempty"`
    Title     string `json:"title,omitempty"`
    Scope     string `json:"scope,omitempty"` // "chapter" or "section"
}
```

If `resolved` is `false` the linkend is not a navigable target anywhere in the
corpus and only `linkend` and `text` are set.

### Export format

`-export DIR` writes one `<id>.json` file per document (see
[./internal/docparse/dump.go](./internal/docparse/dump.go)). Each file is self
sufficient: besides the document it holds each chapter's markdown and prose
and the glossary entries its chapters mention.

```json
{
  "version": 3,
  "document": {
    "id": "...", "title": "...", "summary": "...",
    "distro": {"name": "...", "versions": ["..."]},
    "product": {"name": "...", "versions": ["..."]},
    "license": "...",
    "chapters": [
      {
        "id": "...", "chapter_title": "...", "summary": "...",
        "file": "...",
        "commands": ["..."], "sections": [], "xrefs": ["..."],
        "text": "chapter markdown",
        "prose": "text outside of code blocks"
      }
    ]
  },
  "glossary": [{"term": "...", "whatis": "..."}]
}
```

`version` is the dump format version; files with a different version are
rejected on import.

## MCP interface

The MCP server exposes the following tools:

* `list_documents` — lists all documents in the corpus, optionally filtered
  by product and version
* `read_document` — returns a document's metadata and table of contents, or
  a single chapter's markdown plus the commands it shows, the glossary terms
  it mentions and where its cross references resolve to
* `search_docs` — full text search over all chapters with tf-idf ranking,
  optional fuzzy matching, regex mode and product/version scope
* `find_command` — backward search for where a shell command is mentioned in
  the documentation
* `define_term` — returns the glossary definition of a term

The same data is also available as MCP resources:

* `docs://index` — the document list
* `docs://glossary` — the glossary
* `docs://{doc_id}` — a document's metadata and table of contents
* `docs://{doc_id}/{chapter_id}` — a chapter's markdown

There is a web interface that renders each tool's result in a server-side
template and a JSON API at `/api/{tool}` (plus `/api/corpus` for a corpus
summary); it talks to the same MCP server over an in-memory transport, so it
shows exactly what an MCP client would get.

## Usage

Input is read from DocBook DC, markdown and text files, or from exports:

| Flag | Meaning |
|------|---------|
| `-d DIR` | Read all DocBook DC, markdown and text files from DIR, may be repeated |
| `-f FILE` | Parse a single file, may be repeated or given as arguments (default: stdin) |
| `-import PATH` | Read exported documents from a file or a directory (searched recursively for `*.json`), may be repeated; `-` reads `/usr/share/doc/packages/mcp-server-susedoc.d` |

With no input at all, the packaged exports in
`/usr/share/doc/packages/mcp-server-susedoc.d` are imported.

What to do with the documents:

| Flag | Meaning |
|------|---------|
| (none) | Serve as MCP server on stdin/stdout |
| `-mcp ADDR` | Serve the MCP server over streamable HTTP at `http://ADDR/mcp` |
| `-web ADDR` | Serve the web interface on ADDR |
| `-tls DIR` | Use the certificate and private key `.pem` files in DIR for `-mcp ADDR` and `-web` (https) |
| `-export DIR` | Write the corpus as one `<id>.json` per document to DIR and exit |
| `-dump` | Print the corpus to stdout (yaml, or json with `-json`) |
| `-tree` | Print the raw markdown node tree of a file as json |

Addresses are given as `host[:port]`, the port defaults to 8080.

Other flags: `-debug` logs tool calls, web requests and parsing problems to
stderr, `-failwarn` exits with an error if there were warnings while reading
(like skipped files), `-version`/`-v` prints the version.

Examples:

```
# print the parsed corpus
go run ./cmd/mcp-server-susedoc/ -dump -d $PATH_TO_SUSE_DOC
go run ./cmd/mcp-server-susedoc/ -dump -json -f $PATH_TO_DOC

# MCP server on stdin/stdout
go run ./cmd/mcp-server-susedoc/ -d $PATH_TO_SUSE_DOC

# MCP server over https
go run ./cmd/mcp-server-susedoc/ -mcp localhost:8443 -tls ./certs -d $PATH_TO_SUSE_DOC

# web interface
go run ./cmd/mcp-server-susedoc/ -web localhost:9090 -d $PATH_TO_SUSE_DOC

# export once, serve from the export
go run ./cmd/mcp-server-susedoc/ -export ./export -d $PATH_TO_SUSE_DOC
go run ./cmd/mcp-server-susedoc/ -import ./export
```

## Build

The Go module builds directly with `go build ./cmd/mcp-server-susedoc`.
Releases are cut with GoReleaser when a `v*` tag is pushed; each release
provides one binary per architecture (linux amd64, arm64, ppc64le, s390x)
wrapped in a directory inside a `tar.gz`, plus a checksum and the source.

## License

MIT — see [LICENSE](LICENSE).
