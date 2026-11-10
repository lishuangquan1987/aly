---
title: Introduction
description: What Aly is, what it is made of, which scenarios it fits, and where platform coverage ends.
---

# Introduction

Aly is a **self-contained automatic update system for desktop applications**: publishers push new versions to the server with the publishing tools, and an updater on end-user machines completes the upgrade with a crash-safe atomic replacement. The server, publishing tools, updater, GUI and SDK all live in the same repository and depend on no third-party update framework.

## What It Solves

| Problem | How Aly handles it |
|---|---|
| Old machines cannot be updated | The updater and the Windows GUI are pinned to Go 1.10.8 + 386, so **they run directly on Windows XP**; CI validates the PE header and the ComCtl32 manifest |
| A power cut or crash mid-update bricks the install | Pure directory renames plus a `version.json` state machine; startup detects `applying` and recovers |
| Large updates keep the app down too long | The download stage materializes unchanged files into the new directory, so apply only performs three renames — downtime is measured in seconds |
| How the updater updates itself | `check_self_update` detects and `self_update` performs the replacement; the GUI/SDK delegate, with a failure circuit breaker |
| The host language is locked in | Every command outputs JSON; an optional C# SDK, and any language that can spawn a subprocess, can integrate |

## Five Components

| Component | Role | Stack |
|---|---|---|
| **aly-server** | Project metadata, file storage, REST API | Go 1.25 · Gin · Ent · SQLite |
| **aly-publish** | Publish CLI: configure, diff, stage, push | Go 1.25 · cobra |
| **aly-publish-gui** | Desktop publisher that wraps the CLI | .NET 8 · Avalonia 12.1 · Semi/Ursa |
| **aly-client** | End-user updater | Go 1.10.8 (XP track) / modern Go (cross-platform track) |
| **aly-client-ui** | Optional update window | Windows: walk (XP) · Linux: MyGo + GTK3 |

A host application can either spawn a subprocess and parse the JSON, or integrate the [C# SDK](/en/docs/sdk); the SDK is only an orchestration layer and does not change the protocol.

## Platform Coverage

| Component | Prebuilt | Compilable from source |
|---|---|---|
| Updater / update window | Windows 32-bit (XP+), Linux amd64/arm64 | The same Go source can build for darwin, freebsd and others |
| Server | Windows amd64, Linux amd64/arm64, macOS amd64/arm64 | Other Go targets |
| Publishing tools | Windows amd64, Linux amd64/arm64, macOS amd64/arm64 | Other Go targets |
| Publish GUI | Windows (.NET 8) | Desktop platforms supported by .NET 8 |

## Design Principles

- **Single file, zero dependencies**: the updater and the server are each a single executable; the Linux GUI only depends on the GTK3 preinstalled by the distribution.
- **Protocol before implementation**: a single `{isSuccess, errorMsg, data}` shape; GUI, SDK and updater are decoupled.
- **Configuration ships with the version**: `.updator/shared.json` is uploaded together with each version, so a server address change takes effect with the next update.
- **Everything is verifiable**: MD5 + SHA256 double verification; monotonically increasing versions; [135 end-to-end cases](/en/docs/testing) cover crashes, power loss, concurrency and self-update.

## When to Use It

- Intranet desktop applications (work orders, point of sale, industrial HMI): self-hosted server, no public internet required.
- .NET / C++ applications with many DLLs and resources: directory-level diffing transfers only the changed files.
- Legacy systems that must support Windows XP / 7: something most modern update frameworks cannot do.

::: warning Security boundary
The server currently has no built-in authentication or signing and is designed to be deployed on a trusted intranet. If it is exposed to the public internet, add authentication and access control at the front-end gateway.
:::