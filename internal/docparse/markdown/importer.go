package markdown

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"mcp-server-susedoc/internal/docparse"
)

// Importer reads markdown files. The files of one directory form one
// document, see Build.
type Importer struct{}

// Name returns "markdown".
func (Importer) Name() string { return "markdown" }

// Match reports whether p is a *.md or *.markdown file.
func (Importer) Match(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

// Import reads the files and groups them into documents.
func (Importer) Import(files []docparse.Source) (docparse.Part, error) {
	var segs []Segment
	var errs []error
	for _, f := range files {
		s, err := ReadFile(f.Path, f.Root)
		if err != nil {
			errs = append(errs, fmt.Errorf("skipping %s: %w", f.Path, err))
			continue
		}
		segs = append(segs, s...)
	}
	return Build(segs), errors.Join(errs...)
}
