package docparse

import (
	"regexp"
	"strings"
)

var (
	reServedFence   = regexp.MustCompile("^\\s*(```|~~~)")
	reServedATX     = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	reServedID      = regexp.MustCompile(`\{#[^}\s]+\}\s*$`)
	reServedNumber  = regexp.MustCompile(`^\d+(?:\.\d+)*\.?\s+`)
	reServedAnchors = regexp.MustCompile(`\]\(#([^)\s]+)\)`)
)

// ServedText returns the markdown of the chapter as it is served: headings
// without an explicit id get the {#id} of their chapter or section, so the
// fragment of a reference uri names an anchor in the text, and links to an
// anchor that resolves point to the uri of their target instead. Unresolved
// links and code blocks are left as they are.
func ServedText(text string, ch *Chapter, refs []XrefTarget) string {
	uris := map[string]string{}
	for _, r := range refs {
		if r.Resolved {
			uris[r.Linkend] = r.URI
		}
	}
	lines := strings.Split(text, "\n")
	next := 0 // sections before it got their heading
	fenced := false
	for i, ln := range lines {
		if reServedFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := reServedATX.FindStringSubmatch(ln); m != nil {
			if id := headingID(m[2], i == 0, ch, &next); id != "" && !reServedID.MatchString(m[2]) {
				ln = m[1] + " " + m[2] + " {#" + id + "}"
			}
		}
		lines[i] = reServedAnchors.ReplaceAllStringFunc(ln, func(link string) string {
			if uri, ok := uris[link[3:len(link)-1]]; ok {
				return "](" + uri + ")"
			}
			return link
		})
	}
	return strings.Join(lines, "\n")
}

// headingID returns the id of the chapter heading, the first line of the
// chapter, or of the next section from *next whose title matches the heading.
// Headings are matched by title, so one the importer did not take as a section
// gets no id instead of shifting the ids of the following ones.
func headingID(text string, first bool, ch *Chapter, next *int) string {
	text = strings.TrimSpace(reServedID.ReplaceAllString(text, ""))
	matches := func(title string) bool {
		return text == title || reServedNumber.ReplaceAllString(text, "") == title
	}
	if first && matches(ch.Title) {
		return ch.ID
	}
	for j := *next; j < len(ch.Sections); j++ {
		if matches(ch.Sections[j].Title) {
			*next = j + 1
			return ch.Sections[j].ID
		}
	}
	return ""
}
