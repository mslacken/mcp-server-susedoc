package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"mcp-server-susedoc/internal/debug"
	"mcp-server-susedoc/internal/docparse"
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

// defaultImportDir is the FHS location packaged exports are installed to,
// usually one subdirectory per package holding its <id>.json document files.
const defaultImportDir = "/usr/share/doc/packages/mcp-server-susedoc.d"

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
	var importLocations StringSlice
	flag.Var(&directories, "d", "directory to read all DocBook DC, markdown and text files from, may be given more than once")
	flag.Var(&files, "f", "doc file to parse (DC file, markdown or text), may be given more than once or as arguments (default: stdin)")
	flag.Var(&importLocations, "import", "read documents in the internal format from PATH instead of -d/-f, may be given more than once; PATH is a document file or a directory searched recursively for *.json files; - reads "+defaultImportDir)
	tree := flag.Bool("tree", false, "dump the raw markdown node tree as json")
	asJSON := flag.Bool("json", false, "print documents as json instead of yaml")
	exportDir := flag.String("export", "", "write the corpus in the internal format to the directory PATH, one <id>.json file per document, then exit")
	serve := flag.Bool("mcp", false, "serve the documents as MCP server on stdin/stdout")
	web := flag.Bool("web", false, "serve the documents as a small web interface")
	port := flag.Int("port", 8080, "port the web interface listens on (with -web)")
	debugFlag := flag.Bool("debug", false, "log tool calls, web requests and DocBook parsing problems to stderr")
	flag.Parse()
	debug.SetEnabled(*debugFlag)
	files = append(files, flag.Args()...)

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

	if len(importLocations) == 0 && len(directories) == 0 && len(files) == 0 && (*serve || *web) {
		// without any input serve the packaged exports
		importLocations = append(importLocations, "-")
	}
	if len(importLocations) == 0 && len(directories) == 0 && len(files) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		printCorpus(markdown.BuildDocuments(markdown.SplitSources(data, "stdin")), *asJSON)
		return
	}

	var corpus *docparse.Corpus
	if len(importLocations) > 0 {
		corpus = docparse.NewCorpus()
		for _, location := range importLocations {
			if location == "-" {
				location = defaultImportDir
			}
			imported, err := corpus.Import(location)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error importing corpus:", err)
				os.Exit(1)
			}
			for _, name := range imported {
				debug.Printf("main", "imported %s", name)
			}
		}
		if len(corpus.Documents) == 0 {
			fmt.Fprintf(os.Stderr, "error importing corpus: no documents in %s\n", importLocations.String())
			os.Exit(1)
		}
		corpus.Finish()
	} else {
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
		var err error
		corpus, err = loader.Load(docs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
	}
	if *exportDir != "" {
		if err := corpus.Export(*exportDir); err != nil {
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
