package mcpserver

import (
	"fmt"
	"regexp"
	"strings"
)

var reFence = regexp.MustCompile("^\\s*(```|~~~)")

// textPage is a part of a markdown text as returned by pageText. Offsets and
// sizes are in characters (runes).
type textPage struct {
	Text  string
	Total int // characters of the whole text
	Next  int // offset of the next page, 0 if this is the last one
}

// pageText returns the part of md starting at offset that holds at most max
// characters, or all of the rest for max 0. The cut is placed after a blank
// line outside of a code block if there is one in the second half of the
// page, else at a line start outside of a code block in the second half or,
// if there is none, anywhere in the page, else at any line start and as a
// last resort in the middle of a line. A page that starts or ends
// inside a code block gets the fence reopened or closed, so every page is
// valid markdown on its own; these added fence lines are not counted.
func pageText(md string, offset, max int) (textPage, error) {
	runes := []rune(md)
	total := len(runes)
	if offset < 0 || offset > total || (offset == total && total > 0) {
		return textPage{}, fmt.Errorf("offset %d is outside of the text of %d characters", offset, total)
	}
	end := total
	if max > 0 && total-offset > max {
		end = cutPoint(runes, offset, offset+max)
	}
	text := string(runes[offset:end])
	if open := fenceAt(runes, offset); open != "" {
		text = open + "\n" + text
	}
	if end < total {
		if open := fenceAt(runes, end); open != "" {
			text = strings.TrimRight(text, "\n") + "\n" + closingFence(open) + "\n"
		}
		return textPage{Text: text, Total: total, Next: end}, nil
	}
	return textPage{Text: text, Total: total}, nil
}

// lineInfo describes a line of the text: where it starts, whether it starts
// inside a code block (after an opening fence line) and whether it is blank.
type lineInfo struct {
	start   int
	fenced  bool
	blank   bool
	fenceLn string // the opening fence line of the code block it is in
}

func textLines(runes []rune) []lineInfo {
	var out []lineInfo
	fenced, open := false, ""
	start := 0
	for i := 0; i <= len(runes); i++ {
		if i < len(runes) && runes[i] != '\n' {
			continue
		}
		ln := string(runes[start:i])
		out = append(out, lineInfo{start: start, fenced: fenced, blank: strings.TrimSpace(ln) == "", fenceLn: open})
		t := strings.TrimSpace(ln)
		switch {
		case !fenced && reFence.MatchString(ln):
			fenced, open = true, t
		case fenced && t != "" && strings.Trim(t, open[:1]) == "" && len(t) >= len(closingFence(open)):
			// a run of the opening fence character, at least as long
			fenced, open = false, ""
		}
		start = i + 1
	}
	return out
}

// fenceAt returns the opening fence line of the code block position pos is in,
// else "". The closing fence line still belongs to the block.
func fenceAt(runes []rune, pos int) string {
	open := ""
	for _, l := range textLines(runes) {
		if l.start > pos {
			break
		}
		open = ""
		if l.fenced {
			open = l.fenceLn
		}
	}
	return open
}

// closingFence returns the fence that closes the block opened by the fence
// line open: the same run of backticks or tildes.
func closingFence(open string) string {
	return open[:len(open)-len(strings.TrimLeft(open, open[:1]))]
}

// cutPoint returns where to end a page that starts at from and may run up to
// limit, see pageText.
func cutPoint(runes []rune, from, limit int) int {
	half := from + (limit-from)/2
	var afterBlank, outside, any int
	ls := textLines(runes)
	for i, l := range ls {
		if l.start <= from {
			continue
		}
		if l.start > limit {
			break
		}
		any = l.start
		if !l.fenced {
			outside = l.start
			if i > 0 && ls[i-1].blank && !ls[i-1].fenced {
				afterBlank = l.start
			}
		}
	}
	switch {
	case afterBlank >= half:
		return afterBlank
	case outside >= half:
		return outside
	case outside > from:
		// a short page rather than one cut inside a code block
		return outside
	case any > from:
		return any
	default:
		return limit
	}
}
