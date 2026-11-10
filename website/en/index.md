---
layout: home
description: Lightweight client auto-update system with crash-safe atomic replacement, Windows XP through Linux, one JSON protocol and 135 E2E cases.

hero:
  name: Aly
  text: Lightweight client auto-update system
  tagline: Ship a release with one command; end-user machines upgrade with pure directory renames — from Windows XP to Linux, without ever leaving a half-new, half-old folder.
  actions:
    - theme: brand
      text: Start in 5 minutes
      link: /en/docs/getting-started
    - theme: alt
      text: Module reference
      link: /en/docs/
    - theme: alt
      text: Latest release
      link: https://github.com/lishuangquan1987/aly/releases/latest
      target: _blank

features:
  - icon: 🛡️
    title: Crash-safe atomic replacement
    details: Applying an update is three directory renames. A power cut or crash never leaves a half-new folder; the version.json state machine recovers or rolls back on the next start.
  - icon: ⚡
    title: Seconds-long downtime
    details: Unchanged files are materialized into the new version directory during download, so apply only performs three renames instead of a full copy.
  - icon: 🧩
    title: One JSON protocol
    details: Every updater command emits the same {isSuccess, errorMsg, data} envelope. Hosts can use a language SDK, the optional GUI, or any language that can spawn a process.
  - icon: 🕰️
    title: Native Windows XP support
    details: The updater and Windows GUI are pinned to Go 1.10.8 + 386. CI checks the PE header and ComCtl32 manifest, keeping aging industrial PCs safely updatable.
  - icon: 🌍
    title: Prebuilt for Windows / Linux
    details: The client ships for 32-bit Windows and Linux amd64/arm64; the server and publisher also ship for macOS. Other platforms build from the same source.
  - icon: 🧪
    title: 135 end-to-end cases
    details: A Go-driven E2E harness covers normal flows, crash injection, resumable downloads, rollback, locking and self-update. Each case carries its own preconditions.
---