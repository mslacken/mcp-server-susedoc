// Package loader reads a corpus from a list of files, each with the importer
// matching its type: DocBook assemblies published through DC files, markdown
// files and plain text files.
package loader

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mcp-server-susedoc/internal/debug"
	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/docbook"
	"mcp-server-susedoc/internal/docparse/markdown"
	"mcp-server-susedoc/internal/docparse/text"
)

var debugf = debug.New("loader")

// Importers are the formats files are read as, tried in order.
var Importers = []docparse.Importer{docbook.Importer{}, markdown.Importer{}, text.Importer{}}

// DocFile is an input file and the importer that reads it.
type DocFile struct {
	docparse.Source
	Importer docparse.Importer
}

// importerFor returns the first of Importers that matches p, or nil.
func importerFor(p string) docparse.Importer {
	for _, imp := range Importers {
		if imp.Match(p) {
			return imp
		}
	}
	return nil
}

// File returns the input file p, named directly. Files no importer matches
// are read as markdown.
func File(p string) DocFile {
	imp := importerFor(p)
	if imp == nil {
		imp = markdown.Importer{}
	}
	return DocFile{Source: docparse.Source{Path: p}, Importer: imp}
}

// Find returns the files below dir that an importer matches. A directory with
// entry files of a DirImporter (like the DC files of a DocBook corpus) yields
// just those and is not descended into, as its other files are read through
// them. Hidden directories are skipped. If dir is a file, it is returned as
// File does.
func Find(dir string) ([]DocFile, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []DocFile{File(dir)}, nil
	}
	var out []DocFile
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			for _, imp := range Importers {
				di, ok := imp.(docparse.DirImporter)
				if !ok {
					continue
				}
				if entries := di.EntryFiles(p); len(entries) > 0 {
					for _, e := range entries {
						out = append(out, DocFile{Source: docparse.Source{Path: e, Root: dir}, Importer: imp})
					}
					return filepath.SkipDir
				}
			}
			return nil
		}
		if imp := importerFor(p); imp != nil {
			if _, ok := imp.(docparse.DirImporter); !ok {
				out = append(out, DocFile{Source: docparse.Source{Path: p, Root: dir}, Importer: imp})
			}
		}
		return nil
	})
	return out, err
}

// Load hands the files to their importers, which run in parallel, and builds
// the corpus from what they read. The part of each importer goes where its
// first file is, so that the document order and the first definition winning
// for a glossary term stay deterministic. Unreadable files are skipped with a
// warning; it is an error if no document is left. Once ctx is done loading
// stops and ctx.Err() is returned.
func Load(ctx context.Context, files []DocFile) (*docparse.Corpus, error) {
	var order []docparse.Importer
	sources := map[string][]docparse.Source{}
	for _, f := range files {
		name := f.Importer.Name()
		if _, ok := sources[name]; !ok {
			order = append(order, f.Importer)
		}
		sources[name] = append(sources[name], f.Source)
	}

	parts := make([]docparse.Part, len(order))
	errs := make([]error, len(order))
	var wg sync.WaitGroup
	for i, imp := range order {
		wg.Go(func() {
			srcs := sources[imp.Name()]
			debugf("importing %d %s files", len(srcs), imp.Name())
			parts[i], errs[i] = imp.Import(ctx, srcs)
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c := docparse.NewCorpus()
	for i := range order {
		debug.Warn("loader", errs[i])
		c.Add(parts[i])
	}
	if len(c.Documents) == 0 {
		return nil, fmt.Errorf("no usable documents found")
	}
	debugf("created corpus")
	if err := c.FinishContext(ctx); err != nil {
		return nil, err
	}
	debugf("finished")
	return c, nil
}
