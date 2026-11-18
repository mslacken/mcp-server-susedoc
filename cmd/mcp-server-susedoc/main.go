package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/docbook"
	"mcp-server-susedoc/internal/docparse/loader"
	"mcp-server-susedoc/internal/docparse/markdown"
	"mcp-server-susedoc/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

func printCorpus(c *docparse.Corpus, asJSON bool) {
	var enc []byte
	var err error
	if asJSON {
		enc, err = json.MarshalIndent(c, "", "  ")
	} else {
		enc, err = yaml.Marshal(c)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error encoding output:", err)
		os.Exit(1)
	}
	fmt.Print(string(enc))
}

// exportCorpus writes the corpus in the internal format to path, which may be
// a file or a directory (in which case it is written to <path>/corpus.json).
func exportCorpus(c *docparse.Corpus, path string) error {
	target := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		target = filepath.Join(path, "corpus.json")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	if err := c.Dump(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// importCorpus reads a corpus in the internal format from path, which may be
// the corpus file itself or a directory holding it.
func importCorpus(path string) (*docparse.Corpus, error) {
	target := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		target = filepath.Join(path, "corpus.json")
	}
	f, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	c := docparse.NewCorpus()
	if err := c.Load(f); err != nil {
		return nil, err
	}
	if len(c.Documents) == 0 {
		return nil, fmt.Errorf("no documents in %s", target)
	}
	c.Finish()
	return c, nil
}

var debug bool

type StringSlice []string

func (s *StringSlice) String() string {
	return fmt.Sprint(*s)
}

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var directories StringSlice
	var files StringSlice
	flag.Var(&directories, "d", "directory to read all DocBook DC, markdown and text files from, may be given more than once")
	flag.Var(&files, "f", "doc file to parse (DC file, markdown or text), may be given more than once or as arguments (default: stdin)")
	tree := flag.Bool("tree", false, "dump the raw markdown node tree as json")
	asJSON := flag.Bool("json", false, "print documents as json instead of yaml")
	exportDir := flag.String("export", "", "write the corpus in the internal format to PATH (or PATH/corpus.json if PATH is a directory), then exit")
	importDir := flag.String("import", "", "read a corpus in the internal format from PATH (or PATH/corpus.json if PATH is a directory) instead of -d/-f")
	serve := flag.Bool("mcp", false, "serve the documents as MCP server on stdin/stdout")
	web := flag.Bool("web", false, "serve the documents as a small web interface")
	port := flag.Int("port", 8080, "port the web interface listens on (with -web)")
	debugFlag := flag.Bool("debug", false, "log tool calls, web requests and DocBook parsing problems to stderr")
	flag.Parse()
	debug = *debugFlag
	mcpserver.Debug = debug
	docbook.Debug = debug
	loader.Debug = debug
	files = append(files, flag.Args()...)

	if *importDir != "" {
		dir := *importDir
		if dir == "-" {
			// FHS default location for a packaged export
			dir = "/usr/share/doc/packages/mcp-server-susedoc"
		}
		corpus, err := importCorpus(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error importing corpus:", err)
			os.Exit(1)
		}
		if *web {
			addr := fmt.Sprintf(":%d", *port)
			fmt.Fprintf(os.Stderr, "serving web interface on http://localhost%s/\n", addr)
			if err := http.ListenAndServe(addr, serveWeb(corpus)); err != nil {
				fmt.Fprintln(os.Stderr, "web server:", err)
				os.Exit(1)
			}
			return
		}
		if *serve {
			if err := mcpserver.NewServer(corpus).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
				fmt.Fprintln(os.Stderr, "mcp server:", err)
				os.Exit(1)
			}
			return
		}
		printCorpus(corpus, *asJSON)
		return
	}

	if *tree {
		var data []byte
		var err error
		if len(files) > 0 {
			data, err = os.ReadFile(files[0])
		} else {
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		enc, err := json.MarshalIndent(markdown.BuildTree(data), "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "error encoding json:", err)
			os.Exit(1)
		}
		fmt.Println(string(enc))
		return
	}

	if len(directories) == 0 && len(files) == 0 {
		if *serve || *web {
			fmt.Fprintln(os.Stderr, "-mcp and -web need a file (-f) or a directory (-d)")
			os.Exit(1)
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		printCorpus(markdown.BuildDocuments(markdown.SplitSources(data, "stdin")), *asJSON)
		return
	}

	var docs []loader.DocFile
	for _, dir := range directories {
		found, err := loader.Find(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		docs = append(docs, found...)
	}
	for _, f := range files {
		docs = append(docs, loader.File(f))
	}
	corpus, err := loader.Load(docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading input:", err)
		os.Exit(1)
	}
	if *exportDir != "" {
		if err := exportCorpus(corpus, *exportDir); err != nil {
			fmt.Fprintln(os.Stderr, "error writing export:", err)
			os.Exit(1)
		}
		return
	}
	if *web {
		addr := fmt.Sprintf(":%d", *port)
		fmt.Fprintf(os.Stderr, "serving web interface on http://localhost%s/\n", addr)
		if err := http.ListenAndServe(addr, serveWeb(corpus)); err != nil {
			fmt.Fprintln(os.Stderr, "web server:", err)
			os.Exit(1)
		}
		return
	}
	if *serve {
		if err := mcpserver.NewServer(corpus).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, "mcp server:", err)
			os.Exit(1)
		}
		return
	}
	printCorpus(corpus, *asJSON)
}
