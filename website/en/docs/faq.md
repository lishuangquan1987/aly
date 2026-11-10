---
title: FAQ
description: Common questions about platforms, host languages, downtime, power-loss recovery, rollback, authentication and update rules.
---

# FAQ

## Why keep supporting Windows XP?

Industrial PCs, cashier terminals and hospital equipment still run XP, and those are exactly the machines that most need security updates. Mainstream update frameworks (Squirrel, Velopack, Electron updater) usually require Windows 7+; Aly keeps this path open by pinning Go 1.10.8 + 386, with CI validating the PE header and manifest.

## Must the client host be written in C#?

No. `aly-client` is a standalone executable and every command outputs uniform JSON. Any language that can spawn a subprocess and parse JSON can act as the host; the repository provides C#/TypeScript/Go/Python/Java/Rust/C++/Pascal SDKs plus a VB6/EPL COM wrapper. See [Language SDKs](/en/docs/sdk/languages).

## How long is the application down during an update?

`apply_update` only performs three directory renames — seconds. That is because `download_update` has already materialized unchanged files into the new version directory while the application was running.

## What happens if the power is cut mid-update?

The next startup detects `version_status=applying` and re-runs the renames; the pending directory is validated by file count and content version, and an incomplete one is refused and self-healed, with the next round downloading again. See [Core Concepts](/en/docs/concepts).

## How does the updater update itself?

The new updater enters the main application directory together with the version package; `check_self_update` only detects, while `self_update` performs the replacement in a unified way (take the lock, SHA256, `.new` staging, rollback on failure, circuit breaker after 3 failures within a 30-minute window). See [Self-Update Design](/en/docs/self-update).

## Does rollback need a network connection?

No. `rollback` uses local snapshot directories (`ApplicationFolder_V1.0.0/`) and performs purely local renames. Snapshot retention is controlled by `clean --keep N`.

## Does the server have authentication?

The current version has no built-in authentication or signing and is designed for deployment on a trusted intranet. For public exposure, add authentication at the front-end gateway. See [Server API · Trust boundary](/en/docs/api#trust-boundary-no-authentication).

## Are incremental updates supported?

Yes. The publisher uploads only differing files, and the client downloads only files whose local hash differs; large files are chunked with resumable transfers; a single-file failure never aborts the whole run, and a retry covers only the failed files.

## Why is my first push rejected?

When the server creates a project, the initial version is **V1.0.0**, and `publish_version` requires a version strictly greater than the current latest. Use `V1.0.1` or higher for the first push. The same applies to every later release: an older version number cannot overwrite a newer one.

## Why doesn't `logs/**` in `ignore_folders` take effect?

`ignore_folders` is a **plain prefix match and is case-sensitive**, not a glob; only `logs` ignores the whole directory. Only `ignore_files` supports globs. The client's `un_copy_*` is a separate set of rules that affects materialization only. See [Core Concepts · Configuration Rules](/en/docs/concepts#configuration-rules-ignore-and-un-copy).

## Does the publish GUI support English?

Yes. `AlyPublish` uses Irihi.Lingua and JSON resources; the toolbar switches between Chinese and English, and `--lang zh-CN|en-US` can set the startup language. The choice is stored in `%LOCALAPPDATA%\AlyPublish\settings.json`.

## What dependencies does the Linux GUI need?

Only the GTK3 preinstalled on desktop distributions. MyGo is a pure-Go implementation (no cgo) and the artifact is a single ELF file; machines without a graphical environment can still run the `-finish-self-update` finisher mode.

## How do I change the server address?

Change `server_url` in `.updator/shared.json` and publish again; the configuration is distributed with the version, and the client replaces it automatically on the next update. You can also override it temporarily on the client with `check_update --url`.