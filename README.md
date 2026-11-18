# Present openSUSE/SUSE documentation to an LLM

Having an MCP server that reads openSUSE/SUSE documentation improves the
quality of answers to related questions about these distributions a lot.
This project delivers a server that reads

* DocBook (the DC files SUSE and openSUSE publish)
* markdown
* plain text

and presents every document following Go structs defined in
[./internal/docparse/document.go](./internal/docparse/document.go):

```go
type Document struct {
    ID       string    `json:"id" yaml:"id"`
    Title    string    `json:"title" yaml:"title"`
    Summary  string    `json:"summary" yaml:"summary"`
    Distro   Distro    `json:"distro" yaml:"distro"`
    Product  Product   `json:"product" yaml:"product"`
    Chapters []Chapter `json:"chapters" yaml:"chapters"`
    License  string    `json:"license,omitempty" yaml:"license,omitempty"`
}

type Chapter struct {
    ID         string    `json:"id" yaml:"id"`
    Title      string    `json:"chapter_title" yaml:"chapter_title"`
    Summary    string    `json:"summary,omitempty" yaml:"summary,omitempty"`
    Start      uint      `json:"start" yaml:"start"`
    End        uint      `json:"end" yaml:"end"`
    FilePath   string    `json:"file_path" yaml:"file_path"`
    SourcePath string    `json:"source_path,omitempty" yaml:"source_path,omitempty"`
    Commands   []string  `json:"commands,omitempty" yaml:"commands,omitempty"`
    Sections   []Section `json:"sections,omitempty" yaml:"sections,omitempty"`
    Xrefs      []Xref    `json:"xrefs,omitempty" yaml:"xrefs,omitempty"`
}
```

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

The same data is also available as MCP resources, e.g. `docs://index` for the
document list, `docs://glossary` for the glossary, and
`docs://{doc_id}/{chapter_id}` for a chapter's markdown.

There is a web interface that renders each tool's result in a server-side
template and a JSON API at `/api/{tool}`; it talks to the same MCP server
over an in-memory transport, so it shows exactly what an MCP client would get.

## Usage

Parse documents from a directory or files and print the corpus (yaml by
default, json with `-json`):

```
go run ./cmd/mcp-server-susedoc/ -d $PATH_TO_SUSE_DOC
go run ./cmd/mcp-server-susedoc/ -f $PATH_TO_DOC
go run ./cmd/mcp-server-susedoc/ < $PATH_TO_DOC
```

Serve as an MCP server on stdin/stdout:

```
go run ./cmd/mcp-server-susedoc/ -mcp -d $PATH_TO_SUSE_DOC
```

Serve the web interface (defaults to port 8080):

```
go run ./cmd/mcp-server-susedoc/ -web -d $PATH_TO_SUSE_DOC
go run ./cmd/mcp-server-susedoc/ -web -d $PATH_TO_SUSE_DOC -port 9090
```

With no input, `-mcp` and `-web` read the packaged exports from
`/usr/share/doc/packages/mcp-server-susedoc.d`.

## Build

The Go module builds directly with `go build ./cmd/mcp-server-susedoc`.
Releases are cut with GoReleaser when a `v*` tag is pushed; each release
provides one binary per architecture (linux amd64, arm64, ppc64le, s390x)
wrapped in a directory inside a `tar.gz`, plus a checksum and the source.

## License

MIT — see [LICENSE](LICENSE).
