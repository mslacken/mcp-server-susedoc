package mcpserver

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readPages reads md page by page and checks the invariants of every page: it
// is not larger than max plus the fence lines added, has balanced fences and
// the pages put together give md again.
func readPages(t *testing.T, md string, max int) []textPage {
	t.Helper()
	runes := []rune(md)
	var out []textPage
	var joined strings.Builder
	for offset := 0; ; {
		p, err := pageText(md, offset, max)
		require.NoError(t, err)
		end := p.Next
		if end == 0 {
			end = len(runes)
		}
		require.Greater(t, end, offset, "page makes progress")
		part := string(runes[offset:end])
		joined.WriteString(part)
		assert.Contains(t, p.Text, strings.TrimRight(part, "\n"), "page holds its part of the text")
		assert.LessOrEqual(t, utf8.RuneCountInString(part), max, "page size")
		assert.Equal(t, 0, strings.Count(p.Text, "```")%2, "unbalanced fences in page %q", p.Text)
		out = append(out, p)
		if p.Next == 0 {
			break
		}
		offset = p.Next
	}
	assert.Equal(t, md, joined.String(), "pages put together")
	return out
}

func TestPageTextWhole(t *testing.T) {
	md := "# Title\n\nText.\n"
	for _, max := range []int{0, len(md), 1000} {
		p, err := pageText(md, 0, max)
		require.NoError(t, err)
		assert.Equal(t, textPage{Text: md, Total: len(md)}, p, "max %d", max)
	}
	p, err := pageText("", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, textPage{}, p, "empty text")
	for _, off := range []int{-1, len(md), len(md) + 1} {
		_, err := pageText(md, off, 10)
		assert.Error(t, err, "offset %d", off)
	}
}

func TestPageTextParagraphs(t *testing.T) {
	para := strings.Repeat("word ", 15) + "\n" + strings.Repeat("more ", 15) + "\n" // two lines, 152 chars
	md := strings.Repeat(para+"\n", 5)
	ps := readPages(t, md, 400)
	require.Greater(t, len(ps), 1)
	for _, p := range ps[:len(ps)-1] {
		assert.True(t, strings.HasSuffix(p.Text, "\n\n"), "cut after a blank line: %q", p.Text)
	}
}

func TestPageTextLineFallback(t *testing.T) {
	// no blank lines at all: cut at line starts
	md := strings.Repeat("a line of text\n", 50)
	for _, p := range readPages(t, md, 100) {
		assert.True(t, strings.HasSuffix(p.Text, "\n"), "cut at a line start: %q", p.Text)
	}
	// a single long line: cut in the middle
	long := strings.Repeat("x", 250)
	assert.Len(t, readPages(t, long, 100), 3)
}

func TestPageTextCodeBlocks(t *testing.T) {
	code := "```bash\n" + strings.Repeat("echo something\n", 20) + "```\n"
	md := "Intro paragraph.\n\n" + code + "\nAfter the code.\n"
	// the block fits: never cut inside it, even if the page gets short
	whole := false
	for _, p := range readPages(t, md, len(code)+5) {
		whole = whole || strings.Contains(p.Text, code)
	}
	assert.True(t, whole, "block kept on one page")
	// the block is larger than a page: reopen and close the fence
	ps := readPages(t, md, 100)
	require.Greater(t, len(ps), 2)
	reopened := 0
	for _, p := range ps[1:] {
		if strings.HasPrefix(p.Text, "```bash\n") {
			reopened++
		}
	}
	assert.Positive(t, reopened, "fence reopened on a following page")
	// longer fences are closed with the same fence
	long := "````\n" + strings.Repeat("```\nnested\n", 20) + "````\n"
	p, err := pageText(long, 0, 50)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(p.Text, "\n````\n"), "page = %q", p.Text)
}

func TestPageTextUnicode(t *testing.T) {
	md := strings.Repeat("äöü ✓ text\n\n", 40)
	ps := readPages(t, md, 50)
	for _, p := range ps {
		assert.True(t, utf8.ValidString(p.Text))
		assert.Equal(t, utf8.RuneCountInString(md), p.Total)
	}
}
