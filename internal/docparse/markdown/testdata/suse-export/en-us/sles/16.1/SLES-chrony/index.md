---
title: Synchronizing Time with Chrony | SUSE Linux Enterprise Server 16.1
description: Keep the system clock accurate with chrony
dateCreated: 2026-01-10T00:00+02:00
dateModified: 2026-06-15T00:00+02:00
---

# Synchronizing Time with Chrony

Chrony keeps the system clock in sync with time servers on the network.

## 1 Installing chrony

```
> sudo zypper install chrony
> sudo systemctl enable --now chronyd.service
```

## 2 Using chrony in a container

Containers use the clock of the host, so chrony only runs on the host.

## 3 Legal Notice

Copyright© 2026 SUSE LLC and contributors. All rights reserved.

Permission is granted to copy, distribute and/or modify this document under
the terms of the GNU Free Documentation License, Version 1.2 or (at your
option) version 1.3; with no Invariant Sections, no Front-Cover Texts, and no
Back-Cover Texts.
