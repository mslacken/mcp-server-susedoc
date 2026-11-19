package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	enableMcp := flag.Bool("mcp", false, "serve the documents as MCP server on stdin/stdout")
	web := flag.String("web", "", "serve the documents as a small web interface from the given adress a.g. localhost:8080")
	tlsDir := flag.String("tls", "", "directory holding the tls certificate and private key .pem files; with -web the server is served over https and with -mcp the server is exposed at the http endpoint https://addr/mcp")
	debugFlag := flag.Bool("debug", false, "log tool calls, web requests and DocBook parsing problems to stderr")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.BoolVar(showVersion, "v", false, "shorthand for -version")
	flag.Parse()
	if *showVersion {
		fmt.Println(mcpserver.Version)
		return
	}
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

	if len(importLocations) == 0 && len(directories) == 0 && len(files) == 0 {
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
	// parseAddress turns "host[:port]" into "host:port", defaulting the port
	// to 8080 and dropping a leading http:// or https:// scheme.
	parseAddress := func(in string) (string, error) {
		in = strings.TrimPrefix(in, "http://")
		in = strings.TrimPrefix(in, "https://")
		webSplit := strings.Split(in, ":")
		host := webSplit[0]
		port := "8080"
		if len(webSplit) > 1 {
			port = webSplit[1]
		}
		if _, err := strconv.Atoi(port); err != nil {
			return "", fmt.Errorf("port is invalid: %s", port)
		}
		return fmt.Sprintf("%s:%s", host, port), nil
	}
	// serveHTTP listens on addr and serves handler, over tlsCfg if non-nil
	// (https) or plain http otherwise.
	serveHTTP := func(addr string, handler http.Handler, tlsCfg *tls.Config) error {
		srv := &http.Server{Addr: addr, Handler: handler}
		var ln net.Listener
		var err error
		if tlsCfg != nil {
			ln, err = tls.Listen("tcp", addr, tlsCfg)
		} else {
			ln, err = net.Listen("tcp", addr)
		}
		if err != nil {
			return err
		}
		return srv.Serve(ln)
	}
	if *enableMcp && *tlsDir != "" {
		// MCP over the network at https://addr/mcp, the address taken from
		// -web (defaulting to localhost:8080); without -tls, -mcp serves on
		// stdin/stdout instead.
		addr, err := parseAddress(*web)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			os.Exit(1)
		}
		tlsCfg, err := loadTLS(*tlsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tls:", err)
			os.Exit(1)
		}
		srv := mcpserver.NewServer(corpus)
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
		fmt.Fprintf(os.Stderr, "serving mcp server on https://%s/mcp\n", addr)
		if err := serveHTTP(addr, mux, tlsCfg); err != nil {
			fmt.Fprintln(os.Stderr, "mcp server:", err)
			os.Exit(1)
		}
		return
	}
	if *web != "" {
		addr, err := parseAddress(*web)
		if err != nil {
			fmt.Fprintln(os.Stderr, "web:", err)
			os.Exit(1)
		}
		var tlsCfg *tls.Config
		scheme := "http"
		if *tlsDir != "" {
			tlsCfg, err = loadTLS(*tlsDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, "tls:", err)
				os.Exit(1)
			}
			scheme = "https"
		}
		fmt.Fprintf(os.Stderr, "serving web interface on %s://%s\n", scheme, addr)
		if err := serveHTTP(addr, serveWeb(corpus), tlsCfg); err != nil {
			fmt.Fprintln(os.Stderr, "web server:", err)
			os.Exit(1)
		}
		return
	}
	if *enableMcp {
		if err := mcpserver.NewServer(corpus).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, "mcp server:", err)
			os.Exit(1)
		}
		return
	}
	printCorpus(corpus, *asJSON)
}
