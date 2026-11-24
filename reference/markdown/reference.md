---
title: "Reference Guide | mcp-server-susedoc"
description: >-
  Reference markdown document that exercises every feature of the
  mcp-server-susedoc markdown importer: front matter, chapters, sections,
  cross references, commands, glossary and legal notice.
distro: example distro
version: "23.3"
product: example product
product-version:
  - "1.0"
  - "1.1"
arch: risc6
id: reference-doc
---

# Reference Guide

Applies to example product version 23 patch 3

## Legal Notice

Copyright © 2026 SUSE LLC and contributors. All rights reserved.

Permission is granted to copy, distribute and/or modify this document under
the terms of the GNU Free Documentation License, Version 1.2 or (at your
option) version 1.3; with no Invariant Sections, no Front-Cover Texts, and no
Back-Cover Texts.

## Front matter

This chapter explains the metadata header of a markdown document. The header
is a YAML block between two `---` lines at the very top of the file. As YAML
is a superset of JSON, the block may also hold a JSON object.

### Recognized keys

The importer reads these keys, all others (like `arch` above) are ignored:

| Key | Field | Meaning |
| ----- | ------- | --------- |
| `id` | `Document.ID` | Document ID; without it the name of the directory, or of the file for a lone file, is used |
| `title` | `Document.Title` | Used if the file has no level 1 heading; everything after a `\|` is cut off |
| `description` | `Document.Summary` | Abstract of the document; without it the lead of the first chapter is used |
| `distro` | `Distro.Name` | Distribution the document applies to |
| `version` | `Distro.Versions` | Version(s) of the distribution, a string or a list; quote them so YAML keeps them strings |
| `product` | `Product.Name` | Specific product the document applies to, distinct from the distribution |
| `product-version` | `Product.Versions` | Version(s) of the product, a string or a list |

If `distro` or `version` is missing, an `Applies to DISTRO VERSION` line
within the first lines after the header is used instead.

The license is not a front matter key, it is taken from the
[Legal Notice](#legal-notice) chapter.

### JSON header {#json-header-example}

The same header written as JSON:

```json
---
{"title": "Reference Guide", "description": "Reference markdown document.", "distro": "openSUSE Leap", "version": "16.0", "product": "example product", "product-version": ["1.0", "1.1"]}
---
```

## Document structure

Every directory of markdown files forms one document.
A file that is read on its own forms a document named after
the file.

### Chapters

If a file has a single level 1 heading, every level 2 heading starts a
chapter. With several level 1 headings, they become the chapters instead.
Leading chapter numbers like `3.1` are removed from the title. The chapter
ID is the slug of the title, made unique within the document.

The `file` of a chapter is the file it was written in, relative to the
directory that was read.

### Sections

Headings below the chapter heading stay part of the chapter's markdown. The
paragraphs between the chapter heading and the first of them form the
summary of the chapter.

Every such heading is also listed as a section of the chapter:

| Field | Value |
| ----- | ----- |
| `id` | The explicit `{#id}` of the heading, else the slug of the title |
| `number` | A leading section number like `3.2.1`, removed from the title |
| `title` | The heading text |
| `level` | Depth below the chapter heading, `1` is a direct subsection |
| `file` | Not set for markdown, sections always come from the chapter's file |

#### 3.2.1 Numbered sections

This heading becomes the section `numbered-sections` with the number
`3.2.1` on level 2.

## Cross references

Links to an anchor of the corpus are cross references of the chapter they
are written in. Their targets are listed as `xrefs` of the chapter.

### Link targets

A link `[text](#target)` adds `target` to the xrefs. The target is the ID
of a chapter, like [Commands](#commands), or of a section, like the
[JSON header](#json-header-example) or [Sections](#sections). Every
target is listed once, in order of its first appearance, so a second link
to [Commands](#commands) adds nothing.

Links to other files or web pages, like
[openSUSE](https://www.opensuse.org/), are no cross references.

### Unresolved targets

A target that is no chapter or section anywhere in the corpus, like
[this one](#no-such-target), is still listed. `read_document` reports it as
unresolved.

## Commands

Shell commands are taken from code blocks.

### Prompts

A code block is recognized as command if a line contains following runes followed space rune

- `$`
- `#`
- `>`
regardless what characters are used before the command rune and the `#` can end a command.
This can be obtained by following regular expression `s/.*[$#>] \(.[^#]\)/\1/`.

Example expressions are:

```bash
> sudo zypper refresh
tux@leap> zypper search --installed-only kernel
# systemctl enable --now sshd.service
$ journalctl -u sshd.service \
  --since today
# A comment, not a command
```

The commands found above are

- `sudo zypper refresh`
- `zypper search --installed-only kernel`
- `systemctl enable --now sshd.service`
- `journalctl -u sshd.service --since today`

### Interactive shells

```fooshell
fooshell# configure
fooshell:configure# show
fooshell:configure# commit
```

These must be prefix by their shell commands and become:

- `fooshell configure`
- `fooshell configure show`
- `fooshell configure commit`

## Glossary

Zypper

The command line package manager of openSUSE and SUSE Linux Enterprise.

YaST

The installation and configuration tool of openSUSE and SUSE Linux
Enterprise.

**Front matter**

## No Glossary

dnf

Glossary is defined by chapter

The metadata header at the top of a markdown file, written in YAML or JSON.
