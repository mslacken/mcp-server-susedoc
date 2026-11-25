package docparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAliases(t *testing.T) {
	for term, want := range map[string][]string{
		"CIB (cluster information base)": {"CIB", "cluster information base"},
		"active/active, active/passive":  {"active/active", "active/passive"},
		"Pacemaker":                      {"Pacemaker"},
	} {
		assert.Equal(t, want, aliases(term), "aliases(%q)", term)
	}
}

func TestGlossaryMatchers(t *testing.T) {
	for _, tc := range []struct {
		term, text string
		want       bool
	}{
		{"CIB (cluster information base)", "the CIB is replicated", true},
		{"CIB (cluster information base)", "the Cluster Information Base", true},
		{"CIB (cluster information base)", "cib", false},
		{"DC (designated coordinator)", "cd /etc/dc", false},
		{"ReaR (Relax and Recover)", "the rear node", false},
		{"split brain", "avoid a split-brain scenario", true},
		{"clone", "two clones of it", true},
		{"node", "nodes", true},
		{"node", "nodejs", false},
		{"location constraint", "add a location   constraint", true},
	} {
		got := false
		for _, re := range glossaryMatchers(tc.term) {
			got = got || re.MatchString(tc.text)
		}
		assert.Equal(t, tc.want, got, "%q in %q", tc.term, tc.text)
	}
}

func TestUniqueID(t *testing.T) {
	seen := map[string]int{}
	got := []string{UniqueID(seen, Slug("Overview")), UniqueID(seen, Slug("Overview")), UniqueID(seen, Slug("1.2 What's new?"))}
	assert.Equal(t, []string{"overview", "overview-2", "1-2-what-s-new"}, got)
}

func TestServedText(t *testing.T) {
	ch := &Chapter{ID: "setup-2", Title: "Setup", Sections: []Section{
		{ID: "numbered", Number: "1.2", Title: "Numbered", Level: 1},
		{ID: "explicit", Title: "Explicit", Level: 1},
		{ID: "after", Title: "After", Level: 1},
	}}
	refs := []XrefTarget{
		{Linkend: "numbered", Resolved: true, URI: "docs://doc/setup-2#numbered"},
		{Linkend: "other", Resolved: true, URI: "docs://other/chapter"},
		{Linkend: "dead"},
	}
	text := "## 3 Setup\n\nSee [one](#numbered), [two](#other) and [three](#dead).\n\n" +
		"### 1.2 Numbered\n\n```\n# Not a heading\n[code](#other)\n```\n\n" +
		"### Unknown\n\n### Explicit {#explicit}\n\n### After\n"
	assert.Equal(t, "## 3 Setup {#setup-2}\n\n"+
		"See [one](docs://doc/setup-2#numbered), [two](docs://other/chapter) and [three](#dead).\n\n"+
		"### 1.2 Numbered {#numbered}\n\n```\n# Not a heading\n[code](#other)\n```\n\n"+
		"### Unknown\n\n### Explicit {#explicit}\n\n### After {#after}\n",
		ServedText(text, ch, refs))
}
