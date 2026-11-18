package docparse

// Source is an input file. Root is the directory the file was found in, or ""
// if it was named directly.
type Source struct {
	Path string
	Root string
}

// Importer reads one source format (DocBook, markdown, plain text, ...) into a
// part of a corpus.
type Importer interface {
	// Name is the name of the format, e.g. "docbook".
	Name() string
	// Match reports whether the importer reads the file p, by its name.
	Match(p string) bool
	// Import reads files, all accepted by Match, in their order. Files that
	// cannot be read are skipped: the part holds what was read and the error,
	// if any, joins the reasons the others were skipped.
	Import(files []Source) (Part, error)
}

// DirImporter is an Importer for a format whose files are read through entry
// files at the top level of a directory, like the DC files of a DocBook corpus.
type DirImporter interface {
	Importer
	// EntryFiles returns the paths of the entry files at the top level of dir,
	// or nil if dir holds none. A directory with entry files is not descended
	// into, as its other files are read through them.
	EntryFiles(dir string) []string
}
