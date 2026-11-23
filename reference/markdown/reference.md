---
title: "Reference Guide | mcp-server-susedoc"
description: >-
  Reference markdown document that exercises every feature of the
  mcp-server-susedoc markdown importer: front matter, chapters, sections,
  commands, glossary and legal notice.
product: example product
version: "16.0"
arch: risc6
---

# Reference Guide

Applies to example product version 16

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

The importer reads these keys, all others are ignored:

| Key | Field | Meaning |
| ----- | ------- | --------- |
| `title` | `Document.Title` | Used if the file has no level 1 heading; everything after a `\|` is cut off |
| `description` | `Document.Summary` | Abstract of the document; without it the lead of the first chapter is used |
| `product` | `Distro.Name` | Distribution the document applies to |
| `version` | `Distro.Versions` | Version of the distribution, quote it so YAML keeps it a string |

If `product` or `version` is missing, an `Applies to PRODUCT VERSION` line
within the first lines after the header is used instead.

### JSON header

The same header written as JSON:

```json
---
{"title": "Reference Guide", "description": "Reference markdown document.", "product": "openSUSE Leap", "version": "16.0"}
---
```

## Document structure

Every directory of markdown files forms one document, its ID is the name of
the directory. A file that is read on its own forms a document named after
the file.

### Chapters

If a file has a single level 1 heading, every level 2 heading starts a
chapter. With several level 1 headings, they become the chapters instead.
Leading chapter numbers like `3.1` are removed from the title.

### Sections

Headings below the chapter heading stay part of the chapter's markdown. The
paragraphs between the chapter heading and the first of them form the
summary of the chapter.

## Commands

Shell commands are taken from code blocks if the line starts with a prompt.
Lines without a prompt are output and are ignored.

### Prompts

Fenced command block with language

```bash
> sudo zypper refresh
tux@leap> zypper search --installed-only kernel
# systemctl enable --now sshd.service
$ journalctl -u sshd.service \
  --since today
# A comment, not a command
```

or without

```
echo foo
```

The commands found above are `sudo zypper refresh`,
`zypper search --installed-only kernel`,
`systemctl enable --now sshd.service` and
`journalctl -u sshd.service --since today`.

### Interactive crm shell

```bash
crm(live)# configure
crm(live)configure# show
crm(live)configure# commit
```

These become `crm configure show` and `crm configure commit`.

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
