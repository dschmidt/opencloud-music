---
sidebar_position: 1
slug: /
title: Overview
---

# opencloud-music service reference

:::danger[Proof of concept — does NOT work with released OpenCloud]

opencloud-music depends on OpenCloud changes that are **not yet in any
release** and, in several cases, **not yet upstreamed**. A single released
OpenCloud version that works end-to-end does not exist yet. Until the
required pieces land, the only way to run it is against:

- [`github.com/dschmidt/opencloud`](https://github.com/dschmidt/opencloud/tree/feat/graph-search-full-2) @ `feat/graph-search-full-2`
- [`github.com/dschmidt/libre-graph-api`](https://github.com/dschmidt/libre-graph-api/tree/feat/graph-search-full-2) @ `feat/graph-search-full-2`

Expect broken listings, wrong counts, or empty responses against any stock
OpenCloud build. The repo's
[`bootstrap.sh`](https://github.com/dschmidt/opencloud-music#local-dev-stack-bootstrapsh)
pulls all the moving parts together for a local dev stack.

:::

opencloud-music is an [OpenSubsonic](https://opensubsonic.netlify.app/)-compatible
music server for [OpenCloud](https://opencloud.eu). It translates the Subsonic
API that native music clients speak (Symfonium, play:Sub, Feishin,
Substreamer, …) into OpenCloud Graph search + WebDAV calls — no separate
database, no duplicate index.

- **[Environment variables](configuration/environment-variables)** — every env var parsed by the service at startup.
- **[Example configuration](configuration/example-config)** — YAML config with all defaults.
- **[Deprecations](configuration/deprecations)** — renamed or removed env vars.

For feature status, installation, and operational docs see the
[GitHub repository](https://github.com/dschmidt/opencloud-music#readme).
