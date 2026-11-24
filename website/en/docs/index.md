---
title: Documentation
description: The Aly documentation index — start here, guides, reference, concepts, help.
---

# Documentation

New to Aly? Open [Quick Start](/en/docs/getting-started) and pick the path that matches what you want to do. For what Aly is and which components it contains, read the [introduction](/en/intro).

## Start here

- [Quick Start](/en/docs/getting-started) — three paths: publish, integrate, deploy
- [Publish your first release](/en/docs/start/publish) — install, start the server, ship `V1.0.1` from the GUI
- [Integrate the updater](/en/docs/start/integrate) — launch the updater from your host and verify one update
- [Deploy the server](/en/docs/start/server) — download, start, verify the API
- [Download & install](/en/docs/download) — every asset, package layout, checks and versions

## Guides

- [Package layout & distribution](/en/docs/guide/package) — what is inside the `AlyPublish` package and how to ship the updater
- [Packaging & publishing](/en/docs/publish) — command tree, release flow, version rules
- [Publish GUI](/en/docs/publish/gui) — fields, shortcuts, logs and troubleshooting
- [Deployment & operations](/en/docs/server/deploy) — systemd, reverse proxy, hardening, backups and upgrades
- [Host integration](/en/docs/client/) — lifecycle, JSON contract, exit codes and rollback
- [Update window](/en/docs/client/ui) — `aly-client-ui` modes, arguments and exit codes
- [C# SDK integration](/en/docs/sdk) — the two integration styles and the build-time targets

## Reference

- [aly-client commands](/en/docs/commands) — flags, return fields and failure behavior per command
- [Config commands](/en/docs/publish/commands-config) · [Project commands](/en/docs/publish/commands-project) · [Workflow commands](/en/docs/publish/commands-workflow) — the full `aly-publish` surface
- [Configuration files](/en/docs/client/config) — `client.json`, `shared.json`, `version.json` and the staging area
- [Server API](/en/docs/api) — envelope, error semantics and the endpoint index; [Project API](/en/docs/server/api-project) · [File API](/en/docs/server/api-file) · [Data models](/en/docs/server/api-models)
- [C# SDK API](/en/docs/sdk/api) — classes, methods, enums and data models
- [Language SDKs](/en/docs/sdk/languages) — assets and capability matrix per language

## Concepts

- [Core concepts](/en/docs/concepts) — version state machine, atomic swap, crash recovery, delta download, update lock
- [Architecture](/en/docs/architecture) — data flow of one update and the dual toolchain
- [Self-update design](/en/docs/self-update) — how the updater updates itself
- [Build & cross-platform](/en/docs/build) — how the server, the XP-track updater and the GUI are built
- [Testing](/en/docs/testing) — E2E framework, unit tests and CI

## Help

- [Troubleshooting](/en/docs/troubleshooting) — by symptom: update not triggered, swap failing, lock held
- [FAQ](/en/docs/faq) — platforms, host languages, downtime, authentication and rules
- [Documentation versions](/en/docs/versions) — which code version these pages describe
