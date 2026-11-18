package docbook

// DocSource records where a DocBook document came from.
type DocSource struct {
	DC        string `json:"dc" yaml:"dc"`
	Assembly  string `json:"assembly" yaml:"assembly"`
	Structure string `json:"structure,omitempty" yaml:"structure,omitempty"`
}

// Profile is the profiling a DC file applies (PROFOS, PROFARCH, PROFCONDITION).
type Profile struct {
	OS        []string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch      []string `json:"arch,omitempty" yaml:"arch,omitempty"`
	Condition []string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// DocMeta is the <meta> information of an assembly.
type DocMeta struct {
	Maintainer        string   `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	Series            string   `json:"series,omitempty" yaml:"series,omitempty"`
	Tasks             []string `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Categories        []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Architectures     []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`
	Updated           string   `json:"updated,omitempty" yaml:"updated,omitempty"`
	Title             string   `json:"title,omitempty" yaml:"title,omitempty"` // title for search engines
	Description       string   `json:"description,omitempty" yaml:"description,omitempty"`
	SocialDescription string   `json:"social_description,omitempty" yaml:"social_description,omitempty"`
}

// Revision is an entry of the revision history.
type Revision struct {
	Date        string `json:"date,omitempty" yaml:"date,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Bugtracker is where documentation bugs are reported (docmanager metadata).
type Bugtracker struct {
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	Component string `json:"component,omitempty" yaml:"component,omitempty"`
	Product   string `json:"product,omitempty" yaml:"product,omitempty"`
	Assignee  string `json:"assignee,omitempty" yaml:"assignee,omitempty"`
}

// DocExtra is the DocBook provenance of a document. It hangs off
// docparse.Document.Extras and is not part of the core MCP surface.
type DocExtra struct {
	Source      *DocSource  `json:"source,omitempty" yaml:"source,omitempty"`
	Profile     *Profile    `json:"profile,omitempty" yaml:"profile,omitempty"`
	Subtitle    string      `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`
	Meta        *DocMeta    `json:"meta,omitempty" yaml:"meta,omitempty"`
	Revisions   []Revision  `json:"revisions,omitempty" yaml:"revisions,omitempty"`
	Bugtracker  *Bugtracker `json:"bugtracker,omitempty" yaml:"bugtracker,omitempty"`
	Translation string      `json:"translation,omitempty" yaml:"translation,omitempty"`
}

// DocumentExtra marks DocExtra as a docparse.DocumentExtras implementation.
func (*DocExtra) DocumentExtra() {}

// Procedure is a numbered list of steps. Number is the label of a titled
// procedure, e.g. "19.1" for "Procedure 19.1".
type Procedure struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Number string `json:"number,omitempty" yaml:"number,omitempty"`
	Title  string `json:"title,omitempty" yaml:"title,omitempty"`
	Steps  int    `json:"steps" yaml:"steps"`
}

// Admonition is a note, tip, important, warning or caution box.
type Admonition struct {
	Type  string `json:"type" yaml:"type"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// Link is an external link.
type Link struct {
	URL  string `json:"url" yaml:"url"`
	Text string `json:"text,omitempty" yaml:"text,omitempty"`
}

// Image is an image shown in the chapter; File is relative to the corpus root.
type Image struct {
	File string `json:"file" yaml:"file"`
	Alt  string `json:"alt,omitempty" yaml:"alt,omitempty"`
}

// ChapterExtra is the DocBook structure of a chapter: its number and type, and
// the procedures, admonitions, links, images and includes it contains. It hangs
// off docparse.Chapter.Extra and is not part of the core MCP surface.
type ChapterExtra struct {
	Number        string       `json:"number,omitempty" yaml:"number,omitempty"`
	TopicType     string       `json:"topic_type,omitempty" yaml:"topic_type,omitempty"`
	RenderAs      string       `json:"render_as,omitempty" yaml:"render_as,omitempty"`
	Maintainer    string       `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	LegacySources []string     `json:"legacy_sources,omitempty" yaml:"legacy_sources,omitempty"`
	Procedures    []Procedure  `json:"procedures,omitempty" yaml:"procedures,omitempty"`
	Admonitions   []Admonition `json:"admonitions,omitempty" yaml:"admonitions,omitempty"`
	Links         []Link       `json:"links,omitempty" yaml:"links,omitempty"`
	Images        []Image      `json:"images,omitempty" yaml:"images,omitempty"`
	Includes      []string     `json:"includes,omitempty" yaml:"includes,omitempty"`
}

// ChapterExtra marks ChapterExtra as a docparse.ChapterExtras implementation.
func (x *ChapterExtra) ChapterExtra() {}

// Append-style helpers used by the renderer.
func (x *ChapterExtra) AddLegacySource(u string)   { x.LegacySources = append(x.LegacySources, u) }
func (x *ChapterExtra) AddInclude(f string)        { x.Includes = append(x.Includes, f) }
func (x *ChapterExtra) AddProcedure(p Procedure)   { x.Procedures = append(x.Procedures, p) }
func (x *ChapterExtra) AddAdmonition(a Admonition) { x.Admonitions = append(x.Admonitions, a) }
func (x *ChapterExtra) AddLink(l Link)             { x.Links = append(x.Links, l) }
func (x *ChapterExtra) AddImage(i Image)           { x.Images = append(x.Images, i) }
