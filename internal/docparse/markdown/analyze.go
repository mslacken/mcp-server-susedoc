package markdown

import (
	"regexp"
	"strings"

	"mcp-server-susedoc/internal/docparse"

	"github.com/gomarkdown/markdown/parser"
)

// Analysis is what is extracted from the markdown of a chapter.
type Analysis struct {
	Commands []string // shell commands of the code blocks, without prompt
	Xrefs    []string // targets of the links to an anchor, in order of appearance
	Lead     string   // paragraphs between the chapter heading and the first sub heading
	Prose    string   // running text without code, for glossary matching
}

func init() {
	docparse.ProseOf = func(lines []string) string { return proseOf(parseNodes(lines)) }
}

// Analyze parses the markdown lines of a chapter, starting with its heading.
func Analyze(lines []string) Analysis {
	children := parseNodes(lines)
	return Analysis{Commands: commandsIn(children), Xrefs: xrefsIn(children), Lead: leadText(children), Prose: proseOf(children)}
}

// xrefsIn returns the targets of the links to an anchor, like
// [text](#target), once each, in order of their first appearance.
func xrefsIn(children []node) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(n node)
	walk = func(n node) {
		if n.Type == "Link" {
			if t, ok := strings.CutPrefix(textOf(n, "destination"), "#"); ok && t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range children {
		walk(c)
	}
	return out
}

func parseNodes(lines []string) []node {
	p := parser.NewWithExtensions(parser.CommonExtensions)
	return transform(build(p.Parse([]byte(strings.Join(lines, "\n"))))).Children
}

// leadText returns the paragraphs between the chapter heading and the first
// sub heading.
func leadText(children []node) string {
	var parts []string
	for i, c := range children {
		if c.Type == "Heading" {
			if i == 0 {
				continue
			}
			break
		}
		if c.Type == "Paragraph" {
			if t := strings.TrimSpace(textOf(c, "text")); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, " ")
}

// proseOf returns the running text of the nodes, leaving out code blocks and
// inline code so that commands like `crm configure` are not taken as mentions
// of a glossary term.
func proseOf(children []node) string {
	var b strings.Builder
	var walk func(n node)
	walk = func(n node) {
		switch n.Type {
		case "CodeBlock", "Code", "HTMLBlock", "HTMLSpan":
			return
		case "Text":
			b.WriteString(textOf(n, "text"))
		case "Paragraph", "Heading", "ListItem", "TableCell":
			defer b.WriteString("\n")
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range children {
		walk(c)
	}
	return b.String()
}

var (
	// The prompt expressions are the ones documented in the Commands chapter
	// of reference/markdown/reference.md, keep them in sync.
	// "> cmd", "alice> cmd", "admin@alice> cmd", "$ cmd"; the command is group 2
	reUserPrompt = regexp.MustCompile(`^([[:alnum:]_.-]+@)?[[:alnum:]_.-]*[$>][[:space:]]+([^[:space:]].*)$`)
	// "# cmd" (root prompt), but not "# A comment"
	reRootPrompt = regexp.MustCompile(`^#[[:space:]]+([a-z0-9_./-]+([[:space:]].*)?)$`)
	// interactive crm shell: "crm(live)configure# show"
	reCrmShell = regexp.MustCompile(`^crm\([^)]*\)(\w*)#\s*(.*)$`)

	// callout markers left behind by the html converter: "params foo=bar \1"
	reCallout = regexp.MustCompile(`\\\d+$`)
	// callout markers of the DocBook renderer, anywhere in the line:
	// "Type=forking <1>", "eval <2> \", "vol-create-as POOL <1>NAME"
	reCalloutMark = regexp.MustCompile(`<\d+>`)

	// languages of code blocks holding commands of the system shell
	shellLanguages = map[string]bool{"": true, "bash": true, "sh": true, "shell": true, "console": true, "shell-session": true, "zsh": true}

	crmNavigation = map[string]bool{"cd": true, "up": true, "end": true, "back": true, "quit": true, "exit": true, "bye": true, "ls": true, "help": true}
)

// codeBlock is the text of a code block and its language.
type codeBlock struct {
	text, lang string
}

func commandsIn(children []node) []string {
	var blocks []codeBlock
	var walk func(n node)
	walk = func(n node) {
		if n.Type == "CodeBlock" {
			lang, _, _ := strings.Cut(strings.TrimSpace(textOf(n, "info")), " ")
			blocks = append(blocks, codeBlock{textOf(n, "text"), lang})
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range children {
		walk(c)
	}
	seen := map[string]bool{}
	var out []string
	for _, b := range blocks {
		for _, cmd := range commandsInBlock(b.text, b.lang) {
			if !seen[cmd] {
				seen[cmd] = true
				out = append(out, cmd)
			}
		}
	}
	return out
}

// CommandsInText returns the shell commands of the lines of text that start
// with a prompt, without the prompt and without duplicates.
func CommandsInText(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, cmd := range commandsInBlock(text, "") {
		if !seen[cmd] {
			seen[cmd] = true
			out = append(out, cmd)
		}
	}
	return out
}

// commandsInBlock returns the commands of the code block text. For a block
// in the language of an interactive shell, like ```fooshell, the prompts of
// that shell are recognized too, see shellCommand.
func commandsInBlock(text, lang string) []string {
	var shell *regexp.Regexp
	if !shellLanguages[strings.ToLower(lang)] {
		// "fooshell# configure", "fooshell:configure# show"
		shell = regexp.MustCompile(`^(` + regexp.QuoteMeta(lang) + `)((?::[\w-]+)*)#\s*(.*)$`)
	}
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		cmd, ok := shellCommand(shell, uncallout(lines[i]))
		if !ok {
			cmd, ok = promptCommand(uncallout(lines[i]))
		}
		if !ok {
			continue
		}
		for strings.HasSuffix(cmd, "\\") && i+1 < len(lines) {
			i++
			next := strings.TrimSpace(uncallout(lines[i]))
			if c, ok := promptCommand(next); ok && strings.HasPrefix(next, ">") {
				next = c
			}
			cmd = strings.TrimSpace(strings.TrimSuffix(cmd, "\\")) + " " + next
		}
		cmd = strings.Join(strings.Fields(cmd), " ")
		if significant(cmd) {
			out = append(out, cmd)
		}
	}
	return out
}

// uncallout removes the callout markers from a line of code.
func uncallout(ln string) string {
	ln = strings.TrimRight(reCalloutMark.ReplaceAllString(ln, " "), " \t")
	return reCallout.ReplaceAllString(ln, `\`)
}

// shellCommand returns the command of a line with the prompt of an
// interactive shell matched by re, prefixed by the shell and the levels of the
// prompt: "fooshell:configure# show" is "fooshell configure show".
func shellCommand(re *regexp.Regexp, ln string) (string, bool) {
	if re == nil {
		return "", false
	}
	m := re.FindStringSubmatch(ln)
	if m == nil || strings.TrimSpace(m[3]) == "" {
		return "", false
	}
	words := append([]string{m[1]}, strings.Split(strings.TrimPrefix(m[2], ":"), ":")...)
	return strings.Join(append(words, m[3]), " "), true
}

func promptCommand(ln string) (string, bool) {
	if m := reCrmShell.FindStringSubmatch(ln); m != nil {
		level, args := m[1], strings.Fields(m[2])
		if len(args) == 0 || crmNavigation[args[0]] || (level == "" && len(args) == 1) {
			return "", false
		}
		return strings.Join(append([]string{"crm", level}, args...), " "), true
	}
	if m := reUserPrompt.FindStringSubmatch(ln); m != nil {
		return m[2], true
	}
	if m := reRootPrompt.FindStringSubmatch(ln); m != nil {
		return m[1], true
	}
	return "", false
}

// significant drops stray one word commands like a lone "sudo" or "exit".
func significant(cmd string) bool {
	f := strings.Fields(cmd)
	for len(f) > 0 && f[0] == "sudo" {
		f = f[1:]
	}
	return len(f) >= 2
}
