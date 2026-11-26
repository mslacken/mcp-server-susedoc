package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mcp-server-susedoc/internal/debug"
	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/loader"
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
	var directories, files, importLocations StringSlice
	flag.Var(&directories, "d", "directory to read all DocBook DC, markdown and text files from, may be given more than once")
	flag.Var(&files, "f", "doc file to parse (DC file, markdown or text), may be given more than once or as arguments (default: stdin)")
	flag.Var(&importLocations, "import", "read documents in the internal format from PATH instead of -d/-f, may be given more than once; PATH is a document file or a directory searched recursively for *.json files; - reads "+defaultImportDir)
	exportDir := flag.String("export", "", "write the corpus in the internal format to the directory PATH, one <id>.json file per document, then exit")
	mcpAddr := flag.String("mcp", "", "by default the binary serves the documents as an MCP server over stdin/stdout; with an address argument (a.g. localhost:8080) it listens on that address and is exposed at /mcp over http(s). Giving -mcp without an argument is the same as the default")
	webAddr := flag.String("web", "", "serve the documents as a small web interface from the given adress a.g. localhost:8080")
	tlsDir := flag.String("tls", "", "directory holding the tls certificate and private key .pem files; with -web the server is served over https and with -mcp addr the server is exposed at https://addr/mcp")
	dump := flag.Bool("dump", false, "print the corpus to stdout instead of serving it as an MCP server")
	asJSON := flag.Bool("json", false, "with -dump, print documents as json instead of yaml")
	loadWait := flag.Duration("loadwait", 30*time.Second, "how long a tool call to the stdio MCP server waits for the documents still loading in the background before it fails with an error asking to try again; the loading goes on; 0 waits without a limit")
	loadTimeout := flag.Duration("loadtimeout", 0, "stop loading the documents after this duration, e.g. 2m, and fail all tool calls; 0 means no limit")
	debugFlag := flag.Bool("debug", false, "log tool calls, web requests and DocBook parsing problems to stderr")
	failWarn := flag.Bool("failwarn", false, "exit with an error if there were warnings while reading the documents, like files that were skipped")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.BoolVar(showVersion, "v", false, "shorthand for -version")
	flag.Parse()
	if *showVersion {
		fmt.Println(mcpserver.Version)
		return
	}
	debug.SetEnabled(*debugFlag)
	files = append(files, flag.Args()...)

	// without any input use the packaged exports
	if len(importLocations) == 0 && len(directories) == 0 && len(files) == 0 {
		importLocations = append(importLocations, "-")
	}

	// silence warnings if running as mcp over StdioTransport
	if !debug.Enabled() && *webAddr == "" && *mcpAddr == "" {
		debug.SetSilence(true)
	}
	load := func(ctx context.Context) (*docparse.Corpus, error) {
		if len(importLocations) > 0 {
			corpus := docparse.NewCorpus()
			for _, location := range importLocations {
				if location == "-" {
					location = defaultImportDir
				}
				if err := corpus.Import(ctx, location); err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					debug.Warn("main", err)
				}
			}
			if len(corpus.Documents) == 0 {
				return nil, fmt.Errorf("no documents in %s", importLocations.String())
			}
			if err := corpus.FinishContext(ctx); err != nil {
				return nil, err
			}
			return corpus, nil
		}
		var docs []loader.DocFile
		for _, dir := range directories {
			found, err := loader.Find(dir)
			if err != nil {
				return nil, err
			}
			docs = append(docs, found...)
		}
		for _, f := range files {
			docs = append(docs, loader.File(f))
		}
		return loader.Load(ctx, docs)
	}

	// The default: serve the MCP server on stdin/stdout. It has to answer
	// right away, as clients give up on a server that is slow to start, so
	// the corpus is loaded in the background and tool calls wait for it.
	if *mcpAddr == "" && *webAddr == "" && *exportDir == "" && !*dump && !*failWarn {
		ctx := context.Background()
		pending := mcpserver.LoadInBackground(ctx, *loadTimeout, *loadWait, load)
		go func() {
			if _, err := pending.Wait(ctx, nil); err != nil {
				fmt.Fprintln(os.Stderr, "error reading input:", err)
			}
		}()
		if err := mcpserver.NewLoadingServer(pending).Run(ctx, &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, "mcp server:", err)
			os.Exit(1)
		}
		return
	}

	loadCtx, cancel := context.Background(), context.CancelFunc(func() {})
	if *loadTimeout > 0 {
		loadCtx, cancel = context.WithTimeout(loadCtx, *loadTimeout)
	}
	corpus, err := load(loadCtx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading input:", err)
		os.Exit(1)
	}
	if *failWarn && debug.Warnings() > 0 {
		fmt.Fprintf(os.Stderr, "error reading input: %d warnings\n", debug.Warnings())
		os.Exit(1)
	}
	if *exportDir != "" {
		if err := corpus.Export(*exportDir); err != nil {
			fmt.Fprintln(os.Stderr, "error writing export:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *dump {
		printCorpus(corpus, *asJSON)
		os.Exit(0)
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
	if *mcpAddr != "" {
		// -mcp with an address listens on it and exposes the MCP server at
		// /mcp over http(s) (https when -tls is given).
		addr, err := parseAddress(*mcpAddr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
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
		srv := mcpserver.NewServer(corpus)
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
		fmt.Fprintf(os.Stderr, "serving mcp server on %s://%s/mcp\n", scheme, addr)
		if err := serveHTTP(addr, mux, tlsCfg); err != nil {
			fmt.Fprintln(os.Stderr, "mcp server:", err)
			os.Exit(1)
		}
	}
	if *webAddr != "" {
		addr, err := parseAddress(*webAddr)
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
	}
	// -failwarn without another mode: the corpus is checked, serve it on
	// stdin/stdout.
	if err := mcpserver.NewServer(corpus).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "mcp server:", err)
		os.Exit(1)
	}
}
