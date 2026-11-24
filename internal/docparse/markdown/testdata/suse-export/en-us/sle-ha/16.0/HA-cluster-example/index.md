---
title: Managing Cluster Resources | SUSE Linux Enterprise High Availability 16.0
description: Add, start and stop resources of a High Availability cluster
dateCreated: 2026-03-01T00:00+02:00
---

# Managing Cluster Resources

This article shows how to manage the resources of a cluster with the CRM Shell.

## Managing Cluster Resources

Every service the cluster runs is a resource stored in the CIB.

### Starting a resource

```
> sudo crm resource start web-server
```

### Configuring a resource interactively

```
> sudo crm
crm(live)# configure
crm(live)configure# primitive web-ip IPaddr2 params ip=192.168.1.10
crm(live)configure# commit
crm(live)configure# quit
```

## 2 Putting a node into standby

A node in standby mode runs no resources, see [Starting a resource](#starting-a-resource).

```
> sudo crm node standby alice
```

## 3 Legal Notice

Copyright© 2026 SUSE LLC and contributors. All rights reserved.

Permission is granted to copy, distribute and/or modify this document under
the terms of the GNU Free Documentation License, Version 1.2 or (at your
option) version 1.3; with no Invariant Sections, no Front-Cover Texts, and no
Back-Cover Texts.
