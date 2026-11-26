package markdown

import (
	"context"
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

// Match reports whether p is a *.md, *.markdown or *.mdx file.
func (Importer) Match(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// Import reads the files and groups them into documents.
func (Importer) Import(ctx context.Context, files []docparse.Source) (docparse.Part, error) {
	var segs []Segment
	var errs []error
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return docparse.Part{}, err
		}
		s, err := ReadFile(f.Path, f.Root)
		if err != nil {
			errs = append(errs, fmt.Errorf("skipping %s: %w", f.Path, err))
			continue
		}
		segs = append(segs, s...)
	}
	return Build(segs), errors.Join(errs...)
}
