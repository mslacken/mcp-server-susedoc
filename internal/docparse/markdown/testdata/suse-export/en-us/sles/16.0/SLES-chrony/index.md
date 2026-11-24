---
title: Synchronizing Time with Chrony | SUSE Linux Enterprise Server 16.0
description: Keep the system clock accurate with chrony
dateCreated: 2026-01-10T00:00+02:00
dateModified: 2026-02-01T00:00+02:00
datePublished: 2026-02-03T10:00+02:00
---

# Synchronizing Time with Chrony

Chrony keeps the system clock in sync with time servers on the network.

**Prerequisites**

- `root` privileges or `sudo` access
- Network access to at least one NTP server

## 1 Installing chrony

Install the package and start the service:

```
> sudo zypper install chrony
> sudo systemctl enable --now chronyd.service
```

> **Note**
>
> On a minimal installation chrony may already be installed.

### 1.1 Checking the service

```
> sudo systemctl status chronyd.service
● chronyd.service - NTP client/server
     Active: active (running)
```

## 2 Configuring time servers

Add the servers to `/etc/chrony.conf`, then check them as described in [Checking the service](#checking-the-service):

```
server 0.example.pool.ntp.org iburst
```

To apply the change as `root`:

```
# systemctl restart chronyd.service
# chronyc sources
```

> **Tip**
>
> ```
> > chronyc tracking
> ```

## 3 Legal Notice

Copyright© 2026 SUSE LLC and contributors. All rights reserved.

Permission is granted to copy, distribute and/or modify this document under
the terms of the GNU Free Documentation License, Version 1.2 or (at your
option) version 1.3; with no Invariant Sections, no Front-Cover Texts, and no
Back-Cover Texts.
