---
title: "Linked Guide | mcp-server-susedoc"
description: >-
  Second reference markdown document. It lives in its own directory, so it
  forms a document of its own, and its id is set by the front matter instead
  of being taken from the directory or file name. It links to the Reference
  Guide.
distro: example distro
version: "23.3"
id: linked-reference-doc
---

# Linked Guide

## Document ID

The ID of this document is `linked-reference-doc`, set by the `id` key of
the front matter. Without it, the document would be named `linked` after the
directory it is in. The file name `linking` is not used, as it is only taken
for a lone file.

A file placed directly next to the
[Reference Guide](../reference.md) would become part of that document, so
this file has a directory of its own.

## Links to other documents

Cross references are resolved across the whole corpus, so this document can
link to the chapters and sections of the Reference Guide.

### Chapter targets

Links to the chapters [Front matter](#front-matter) and
[Commands](#commands) of the Reference Guide are cross references of this
chapter.

### Section targets

Links to sections work the same, like the
[JSON header](#json-header-example) or the
[Interactive shells](#interactive-shells).

The link to the [Reference Guide](../reference.md) file itself is no cross
reference, as it does not point to an anchor.
