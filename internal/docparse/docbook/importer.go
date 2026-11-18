package docbook

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"mcp-server-susedoc/internal/docparse"
)

// Importer reads DC files. DC files in the same directory share one Loader,
// so that common topics and entity files are parsed once.
type Importer struct{}

// Name returns "docbook".
func (Importer) Name() string { return "docbook" }

// Match reports whether p names a DC file.
func (Importer) Match(p string) bool { return IsDCFile(p) }

// EntryFiles returns the paths of the DC files at the top level of dir.
func (Importer) EntryFiles(dir string) []string {
	var out []string
	for _, name := range DCFiles(dir) {
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

// Import reads the DC files in parallel and returns their documents in file
// order.
func (Importer) Import(files []docparse.Source) (docparse.Part, error) {
	loaders := map[string]*Loader{}
	for _, f := range files {
		if dir := filepath.Dir(f.Path); loaders[dir] == nil {
			loaders[dir] = NewLoader(dir)
		}
	}
	parts := make([]docparse.Part, len(files))
	errs := make([]error, len(files))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range jobs {
				f := files[i]
				debugf("reading %s", f.Path)
				parts[i], errs[i] = loaders[filepath.Dir(f.Path)].Read(filepath.Base(f.Path))
				if errs[i] != nil {
					errs[i] = fmt.Errorf("skipping %s: %w", f.Path, errs[i])
				}
			}
		})
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var out docparse.Part
	for i := range parts {
		out.Merge(parts[i])
	}
	return out, errors.Join(errs...)
}
