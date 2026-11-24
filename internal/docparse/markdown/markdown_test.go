package markdown

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"mcp-server-susedoc/internal/docparse"

	"github.com/stretchr/testify/assert"
)

func TestProseSkipsCode(t *testing.T) {
	prose := proseOf(parseNodes(strings.Split("## Title\n\nUse `crm` here.\n\n```\n# crm configure\n```\n", "\n")))
	assert.NotContains(t, prose, "crm")
	assert.Contains(t, prose, "Use  here.")
}

func importTree(t *testing.T, files map[string]string) *docparse.Corpus {
	t.Helper()
	root := t.TempDir()
	var paths []string
	for p, content := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, full)
	}
	sort.Strings(paths)
	var srcs []docparse.Source
	for _, p := range paths {
		if !(Importer{}).Match(p) {
			t.Fatalf("%s is not read as markdown", p)
		}
		srcs = append(srcs, docparse.Source{Path: p, Root: root})
	}
	part, err := Importer{}.Import(srcs)
	if err != nil {
		t.Fatal(err)
	}
	c := docparse.NewCorpus()
	c.Add(part)
	c.Finish()
	return c
}

type wantChapter struct {
	ID, Title, Text, Summary string
	Commands                 []string
}

type wantDoc struct {
	ID, Title, Summary string
	Chapters           []wantChapter
}

func checkDoc(t *testing.T, c *docparse.Corpus, want wantDoc) {
	t.Helper()
	d := c.Document(want.ID)
	if d == nil {
		var ids []string
		for _, d := range c.Documents {
			ids = append(ids, d.ID)
		}
		t.Fatalf("document %q not found in %q", want.ID, ids)
	}
	if d.Title != want.Title {
		t.Errorf("%s: title %q, want %q", d.ID, d.Title, want.Title)
	}
	if want.Summary != "" && d.Summary != want.Summary {
		t.Errorf("%s: summary %q, want %q", d.ID, d.Summary, want.Summary)
	}
	var got, wantIDs []string
	for _, ch := range d.Chapters {
		got = append(got, ch.ID)
	}
	for _, ch := range want.Chapters {
		wantIDs = append(wantIDs, ch.ID)
	}
	if !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("%s: chapters %q, want %q", d.ID, got, wantIDs)
	}
	for i, w := range want.Chapters {
		ch := &d.Chapters[i]
		if w.Title != "" && ch.Title != w.Title {
			t.Errorf("%s/%s: title %q, want %q", d.ID, ch.ID, ch.Title, w.Title)
		}
		if w.Text != "" && c.Text(ch) != w.Text {
			t.Errorf("%s/%s: text\n%q\nwant\n%q", d.ID, ch.ID, c.Text(ch), w.Text)
		}
		if w.Summary != "" && ch.Summary != w.Summary {
			t.Errorf("%s/%s: summary %q, want %q", d.ID, ch.ID, ch.Summary, w.Summary)
		}
		if len(ch.Commands) != 0 || len(w.Commands) != 0 {
			if !reflect.DeepEqual(ch.Commands, w.Commands) {
				t.Errorf("%s/%s: commands %q, want %q", d.ID, ch.ID, ch.Commands, w.Commands)
			}
		}
	}
}

func TestDocumentationStyles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []wantDoc
	}{{
		name:  "suse",
		files: map[string]string{"HA-guide/index.md": "---\ntitle: HA Guide | SUSE Documentation\ndescription: Set up a cluster.\nproduct: SUSE Linux Enterprise High Availability\nversion: \"16.0\"\n---\n\n# HA Guide\n\nApplies to SUSE Linux Enterprise High Availability 16.0\n\n## 1 Installing\n\nInstall the packages.\n\n```\n> sudo zypper install -t pattern ha_sles\n# crm cluster init\n```\n\n## 2 Glossary\n\nCIB (cluster information base)\n\nThe cluster configuration.\n"},
		want: []wantDoc{{ID: "HA-guide", Title: "HA Guide", Summary: "Set up a cluster.", Chapters: []wantChapter{
			{ID: "installing", Title: "Installing", Text: "## 1 Installing\n\nInstall the packages.\n\n```\n> sudo zypper install -t pattern ha_sles\n# crm cluster init\n```",
				Commands: []string{"sudo zypper install -t pattern ha_sles", "crm cluster init"}},
		}}},
	}, {
		name:  "readme",
		files: map[string]string{"proj/README.md": "# My Project\n\nMy Project syncs files between hosts.\n\n## Installation\n\n```bash\n# install from the distribution\nsudo apt install myproj\nmyproj --version\n```\n\n## Usage\n\nRun it:\n\n```console\n$ myproj sync /src /dst\nsynced 3 files\n```\n"},
		want: []wantDoc{{ID: "proj", Title: "My Project", Summary: "My Project syncs files between hosts.", Chapters: []wantChapter{
			{ID: "my-project", Title: "My Project", Text: "# My Project\n\nMy Project syncs files between hosts.", Summary: "My Project syncs files between hosts."},
			{ID: "installation", Commands: []string{"sudo apt install myproj", "myproj --version"}},
			{ID: "usage", Commands: []string{"myproj sync /src /dst"}},
		}}},
	}, {
		name: "docusaurus",
		files: map[string]string{
			"site/install.md": "---\ntitle: Installation\nsidebar_position: 2\n---\n\nIntro text before any heading.\n\n## Requirements\n\n:::note\nYou need root.\n:::\n\n## Steps {#steps-anchor}\n\n```sh\ncurl -sfL https://get.example.com | \\\n  sh -\n```\n",
			"site/config.mdx": "---\ntitle: Configuration\n---\nimport Tabs from '@theme/Tabs';\n\n## Config file\n\n<Tabs><TabItem value=\"a\">\n\nEdit `/etc/x.yaml`.\n\n</TabItem></Tabs>\n",
		},
		want: []wantDoc{{ID: "site", Title: "Configuration", Chapters: []wantChapter{
			{ID: "config-file", Title: "Config file"},
			{ID: "installation", Title: "Installation", Text: "Intro text before any heading."},
			{ID: "requirements", Summary: "You need root."},
			{ID: "steps-anchor", Title: "Steps", Commands: []string{"curl -sfL https://get.example.com | sh -"}},
		}}},
	}, {
		name:  "setext",
		files: map[string]string{"guide/page.md": "Guide\n=====\n\nSome intro.\n\nInstall\n-------\n\nRun the installer.\n\nConfigure\n---------\n\nEdit the file.\n"},
		want: []wantDoc{{ID: "guide", Title: "Guide", Chapters: []wantChapter{
			{ID: "guide", Text: "Guide\n=====\n\nSome intro."},
			{ID: "install", Title: "Install", Text: "Install\n-------\n\nRun the installer."},
			{ID: "configure", Title: "Configure", Text: "Configure\n---------\n\nEdit the file."},
		}}},
	}, {
		name:  "hugo",
		files: map[string]string{"hugo/page.md": "+++\ntitle = \"Hugo Page\"\ndescription = 'A page.'\ndraft = false\n+++\n\n## First\n\nText.\n"},
		want: []wantDoc{{ID: "hugo", Title: "Hugo Page", Summary: "A page.", Chapters: []wantChapter{
			{ID: "first", Text: "## First\n\nText."},
		}}},
	}, {
		name:  "single page",
		files: map[string]string{"notes/page.md": "# Only a title\n\nA short page about the kubelet.\n"},
		want: []wantDoc{{ID: "notes", Title: "Only a title", Chapters: []wantChapter{
			{ID: "only-a-title", Text: "# Only a title\n\nA short page about the kubelet."},
		}}},
	}, {
		name:  "plain",
		files: map[string]string{"misc/release-notes.md": "Version 2 fixes the scheduler.\n"},
		want: []wantDoc{{ID: "misc", Title: "release-notes", Chapters: []wantChapter{
			{ID: "release-notes", Title: "release-notes", Text: "Version 2 fixes the scheduler."},
		}}},
	}, {
		name:  "h3 sections",
		files: map[string]string{"deep/page.md": "# T\n\n### One\n\nFirst.\n\n### Two\n\nSecond.\n"},
		want: []wantDoc{{ID: "deep", Title: "T", Chapters: []wantChapter{
			{ID: "one", Text: "### One\n\nFirst."},
			{ID: "two", Text: "### Two\n\nSecond."},
		}}},
	}, {
		name:  "numbers",
		files: map[string]string{"blog/post.md": "# Post\n\n## 2022\n\nA year.\n\n## 2023 Roadmap\n\nPlans.\n\n## 1.2 Numbered\n\nText.\n"},
		want: []wantDoc{{ID: "blog", Title: "Post", Chapters: []wantChapter{
			{ID: "2022", Title: "2022"},
			{ID: "2023-roadmap", Title: "2023 Roadmap"},
			{ID: "numbered", Title: "Numbered"},
		}}},
	}, {
		name: "suse placeholders",
		files: map[string]string{
			"empty/index.md":  "---\ntitle: Documentation Index for target\n---\n\n",
			"broken/index.md": "---\ntitle: Documentation Index for target\n---\n\next/markdown\" href=\"../docs/x/index.md\">\n",
			"dup/index.md":    "---\ntitle: Agama\n---\n\n# Using Agama\n\nAbstract of the page.\n\n## Using Agama\n\nThe chapter.\n",
		},
		want: []wantDoc{
			{ID: "empty", Title: "", Chapters: nil},
			{ID: "broken", Title: "Documentation Index for target", Chapters: nil},
			{ID: "dup", Title: "Using Agama", Chapters: []wantChapter{
				{ID: "using-agama-2", Text: "# Using Agama\n\nAbstract of the page."},
				{ID: "using-agama", Text: "## Using Agama\n\nThe chapter."},
			}},
		},
	}, {
		name:  "crlf",
		files: map[string]string{"win/page.md": "---\r\ntitle: Windows\r\ndescription: Line endings.\r\n---\r\n\r\n## Part\r\n\r\nText.\r\n"},
		want: []wantDoc{{ID: "win", Title: "Windows", Summary: "Line endings.", Chapters: []wantChapter{
			{ID: "part", Text: "## Part\n\nText."},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			c := importTree(t, tc.files)
			if len(c.Documents) != len(tc.want) {
				t.Errorf("%d documents, want %d", len(c.Documents), len(tc.want))
			}
			for _, w := range tc.want {
				checkDoc(t, c, w)
			}
		})
	}
}

func TestSuseGlossary(t *testing.T) {
	c := importTree(t, map[string]string{"guide/index.md": "# Guide\n\n## Setup\n\nThe CIB.\n\n## Glossary\n\nCIB (cluster information base)\n\nThe cluster configuration.\n"})
	if e := c.DefineTerm("CIB", nil); len(e) != 1 || e[0].WhatIs != "The cluster configuration." {
		t.Errorf("glossary = %+v", e)
	}
	checkDoc(t, c, wantDoc{ID: "guide", Title: "Guide", Chapters: []wantChapter{{ID: "setup"}}})
}

func TestShellCommands(t *testing.T) {
	for _, tc := range []struct {
		info, code string
		want       []string
	}{
		{"", "> sudo zypper ref\n# zypper up\nLoading repository data...", []string{"sudo zypper ref", "zypper up"}},
		{"console", "$ kubectl get nodes\nNAME STATUS\nnode1 Ready", []string{"kubectl get nodes"}},
		{"bash", "$ make build\n$ make install", []string{"make build", "make install"}},
		{"bash", "# fetch it\ncurl -LO https://example.com/x.tgz\n\ntar xzf x.tgz \\\n  -C /opt\n", []string{"curl -LO https://example.com/x.tgz", "tar xzf x.tgz -C /opt"}},
		{"sh", "export K=1\nsystemctl enable --now k3s", []string{"export K=1", "systemctl enable --now k3s"}},
		{"Shell", "kubectl apply -f x.yaml", []string{"kubectl apply -f x.yaml"}},
		{"bash", "cat <<EOF > /etc/x.conf\nkey: value\nother thing\nEOF\nsystemctl restart x", []string{"cat <<EOF > /etc/x.conf", "systemctl restart x"}},
		{"bash", "cat <<-'END'\n\tnot a command\n\tEND\nls -la /tmp", []string{"cat <<-'END'", "ls -la /tmp"}},
		{"bash", "for f in *.crt; do\n  stat -c %a \"$f\"\ndone\nif [ -n \"$X\" ]; then\n  echo set\nfi\ncase $1 in\n  start) run_it now;;\nesac", []string{`stat -c %a "$f"`, "echo set"}},
		{"bash", "--flag value (helm) Customized configuration\n- JavaScript\nSet kubectl context to kind\nKUBECONFIG=/x kubectl get pods", []string{"KUBECONFIG=/x kubectl get pods"}},
		{"bash", "check() {\n  grep -q x /etc/y\n}\nmkdocs serve --livereload # (1)!", []string{"grep -q x /etc/y", "mkdocs serve --livereload"}},
		{"", "kubectl get nodes", nil},
		{"yaml", "apiVersion: v1\nkind: Pod", nil},
		{"console", "kubectl get nodes", nil},
		{"bash", "ls\nexit", nil},
	} {
		if got := commandsInCode(tc.code, tc.info); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %q:\ngot  %q\nwant %q", tc.info, tc.code, got, tc.want)
		}
	}
}

func TestSuseExport(t *testing.T) {
	dir := filepath.Join("testdata", "suse-export")
	var srcs []docparse.Source
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			srcs = append(srcs, docparse.Source{Path: p, Root: dir})
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	part, err := Importer{}.Import(srcs)
	if err != nil {
		t.Fatal(err)
	}
	c := docparse.NewCorpus()
	c.Add(part)
	c.Finish()

	var ids []string
	for _, d := range c.Documents {
		ids = append(ids, d.ID)
	}
	if want := []string{"HA-cluster-example", "SLES-chrony", "SLES-broken-index", "SLES-chrony-2", "SLES-placeholder"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("documents = %q, want %q", ids, want)
	}

	checkDoc(t, c, wantDoc{ID: "HA-cluster-example", Title: "Managing Cluster Resources", Summary: "Add, start and stop resources of a High Availability cluster", Chapters: []wantChapter{
		{ID: "managing-cluster-resources-2", Title: "Managing Cluster Resources", Text: "# Managing Cluster Resources\n\nThis article shows how to manage the resources of a cluster with the CRM Shell."},
		{ID: "managing-cluster-resources", Title: "Managing Cluster Resources", Summary: "Every service the cluster runs is a resource stored in the CIB.",
			Commands: []string{"sudo crm resource start web-server", "crm configure primitive web-ip IPaddr2 params ip=192.168.1.10", "crm configure commit"}},
		{ID: "putting-a-node-into-standby", Title: "Putting a node into standby", Commands: []string{"sudo crm node standby alice"}},
	}})
	checkDoc(t, c, wantDoc{ID: "SLES-chrony", Title: "Synchronizing Time with Chrony", Summary: "Keep the system clock accurate with chrony", Chapters: []wantChapter{
		{ID: "synchronizing-time-with-chrony", Text: "# Synchronizing Time with Chrony\n\nChrony keeps the system clock in sync with time servers on the network.\n\n**Prerequisites**\n\n- `root` privileges or `sudo` access\n- Network access to at least one NTP server"},
		{ID: "installing-chrony", Commands: []string{"sudo zypper install chrony", "sudo systemctl enable --now chronyd.service", "sudo systemctl status chronyd.service"}},
		{ID: "configuring-time-servers", Commands: []string{"systemctl restart chronyd.service", "chronyc sources", "chronyc tracking"}},
	}})
	checkDoc(t, c, wantDoc{ID: "SLES-chrony-2", Title: "Synchronizing Time with Chrony", Chapters: []wantChapter{
		{ID: "synchronizing-time-with-chrony"},
		{ID: "installing-chrony", Commands: []string{"sudo zypper install chrony", "sudo systemctl enable --now chronyd.service"}},
		{ID: "using-chrony-in-a-container"},
	}})
	checkDoc(t, c, wantDoc{ID: "SLES-placeholder", Title: ""})
	checkDoc(t, c, wantDoc{ID: "SLES-broken-index", Title: "Documentation Index for target"})

	for _, id := range []string{"HA-cluster-example", "SLES-chrony", "SLES-chrony-2"} {
		if l := c.Document(id).License; l != "GFDL-1.2-no-invariants-only OR GFDL-1.3-no-invariants-only" {
			t.Errorf("%s: license = %q", id, l)
		}
	}
	for term, def := range map[string]string{
		"CIB":     "The XML representation of the whole cluster configuration and status.",
		"standby": "A node state in which the node keeps running but does not host resources.",
	} {
		if e := c.DefineTerm(term, nil); len(e) != 1 || e[0].WhatIs != def {
			t.Errorf("glossary %s = %+v", term, e)
		}
	}

	covered := map[string]map[int]bool{}
	for _, d := range c.Documents {
		for _, ch := range d.Chapters {
			if covered[ch.File] == nil {
				covered[ch.File] = map[int]bool{}
			}
			for l := ch.Start; l <= ch.End; l++ {
				covered[ch.File][int(l)] = true
			}
		}
	}
	for _, src := range srcs {
		rel, _ := filepath.Rel(dir, src.Path)
		if lost := lostLines(t, src.Path, covered[filepath.ToSlash(rel)]); len(lost) > 0 {
			t.Errorf("%s: lines in no chapter: %q", src.Path, lost)
		}
	}
}

var (
	reRealATX  = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
	reRealSkip = regexp.MustCompile(`^(?:(?:import|export)\s|<!--.*-->$|#\s|=+$|<)|\bhref="`)
)

func lostLines(t *testing.T, p string, covered map[int]bool) []string {
	t.Helper()
	base := strings.ToLower(filepath.Base(p))
	if strings.Contains(base, "license") || strings.Contains(base, "glossary") || strings.HasPrefix(base, "rh-") {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	body := 0
	if len(lines) > 0 && (lines[0] == "---" || lines[0] == "+++") {
		for j := 1; j < len(lines); j++ {
			if lines[j] == lines[0] {
				body = j + 1
				break
			}
		}
	}
	for _, ln := range lines[body:] {
		if m := reRealATX.FindStringSubmatch(strings.TrimSpace(ln)); m != nil {
			if reHistory.MatchString(m[1]) {
				return nil
			}
			break
		}
	}
	var lost []string
	skipSection, fenced := false, false
	for i := body; i < len(lines); i++ {
		ln := strings.TrimSpace(lines[i])
		if reFence.MatchString(ln) {
			fenced = !fenced
		}
		if m := reRealATX.FindStringSubmatch(ln); m != nil && !fenced {
			skipSection = glossaryHeading.MatchString(m[1]) || docparse.IsLegalNotice(m[1])
		}
		if ln == "" || skipSection || covered[i+1] || reRealSkip.MatchString(ln) {
			continue
		}
		lost = append(lost, ln)
	}
	return lost
}
