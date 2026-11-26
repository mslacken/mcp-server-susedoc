package text

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"mcp-server-susedoc/internal/docparse"
)

// Importer reads plain text files, one document each.
type Importer struct{}

// Name returns "text".
func (Importer) Name() string { return "text" }

// Match reports whether p is a *.txt or *.text file.
func (Importer) Match(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".txt" || ext == ".text"
}

// Import reads the files in order.
func (Importer) Import(ctx context.Context, files []docparse.Source) (docparse.Part, error) {
	var out docparse.Part
	var errs []error
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return docparse.Part{}, err
		}
		p, err := ReadFile(f.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("skipping %s: %w", f.Path, err))
			continue
		}
		out.Merge(p)
	}
	return out, errors.Join(errs...)
}
