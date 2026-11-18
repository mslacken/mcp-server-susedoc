package docparse

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDumpLoad(t *testing.T) {
	c := NewCorpus()
	c.Add(Part{
		Documents: []Document{{
			ID: "guide", Title: "Guide", Distro: Distro{Name: "SLES", Versions: []string{"16.0"}},
			Chapters: []Chapter{{
				ID: "intro", Title: "Intro", Start: 2, End: 3, FilePath: "guide.md",
				Commands: []string{"zypper in foo"}, LinesKey: "guide.md", Prose: "Install foo.",
			}},
		}},
		Glossary: []GlossaryEntry{{Term: "Foo", WhatIs: "A package."}, {Term: "Bar", WhatIs: "Not mentioned."}},
		Lines:    map[string][]string{"guide.md": {"preamble", "# Intro", "Install foo."}},
	})
	c.Finish()

	var buf bytes.Buffer
	if err := c.DumpDocument(&buf, &c.Documents[0]); err != nil {
		t.Fatal(err)
	}
	got := NewCorpus()
	if err := got.Load(&buf); err != nil {
		t.Fatal(err)
	}
	got.Finish()

	ch := &got.Documents[0].Chapters[0]
	if text := got.Text(ch); text != "# Intro\nInstall foo." {
		t.Errorf("text = %q", text)
	}
	if ch.Prose != "Install foo." || !reflect.DeepEqual(ch.Commands, []string{"zypper in foo"}) {
		t.Errorf("chapter = %+v", ch)
	}
	if !reflect.DeepEqual(got.Documents[0].Distro, c.Documents[0].Distro) {
		t.Errorf("distro = %+v", got.Documents[0].Distro)
	}
	if e := got.DefineTerm("foo", nil); len(e) != 1 || e[0].WhatIs != "A package." {
		t.Errorf("glossary = %+v", e)
	}
	if e := got.DefineTerm("bar", nil); len(e) != 0 {
		t.Errorf("unmentioned glossary term dumped: %+v", e)
	}
}
