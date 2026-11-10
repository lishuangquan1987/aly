---
layout: home

hero:
  name: Aly
  text: Lightweight Client Auto-Update System
  tagline: Ship new versions with a single command; end-user machines upgrade with crash-safe atomic replacement — covering Windows XP all the way to Linux / macOS / FreeBSD.
  actions:
    - theme: brand
      text: Quick Start
      link: /en/docs/getting-started
    - theme: alt
      text: Introduction
      link: /en/intro
    - theme: alt
      text: GitHub
      link: https://github.com/lishuangquan1987/aly

features:
  - icon: 🛡️
    title: Crash-Safe Atomic Replacement
    details: Applying an update is pure directory renames. A power cut or crash at any moment never leaves a half-new half-old directory; on restart the version.json state machine recovers or rolls back automatically.
  - icon: ⚡
    title: Seconds-Long Downtime Window
    details: Unchanged files are materialized into the new version directory during download, so apply only performs three renames — the "app is down" window drops from minutes to seconds.
  - icon: 🧩
    title: One Protocol, Any Host
    details: Every updater command outputs the same JSON protocol. Hosts can be a C# client (NuGet SDK), an optional GUI, or anything that can spawn a subprocess.
  - icon: 🕰️
    title: Windows XP Compatible
    details: The updater and Windows GUI are pinned to Go 1.10.8 + 386, with CI validating PE headers and manifests — aging industrial PCs and POS terminals stay securely updatable.
  - icon: 🌍
    title: Truly Cross-Platform
    details: The same update logic compiles to windows 386/amd64, linux amd64/arm64, darwin amd64/arm64 and freebsd amd64; the Linux GUI renders natively in pure Go.
  - icon: 🧪
    title: 135 End-to-End Cases
    details: A Go-driven E2E framework covers normal flows, crash injection, resumable downloads, rollback and self-update — every case self-contained and independently reproducible.
---
