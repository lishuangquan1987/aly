---
title: Documentation Versions
description: Which code version this documentation tracks, and how to read the docs for an older release.
---

# Documentation Versions

This site documents `master` and the latest release. Every page describes the current implementation and is not guaranteed to match an older binary word for word.

## Read the docs for an older release

Documentation lives in the same repository and tag as the code. To read the docs for a specific version, open that tag on GitHub and browse `website/docs/`:

1. Find the deployed tag on [Releases](https://github.com/lishuangquan1987/aly/releases);
2. Open `https://github.com/lishuangquan1987/aly/tree/<tag>/website/docs`;
3. To render locally, check out that tag and run `cd website && pnpm install && pnpm dev`.

For a given version, the authoritative CLI parameters and return fields are that tag's `COMMANDS.md` (Chinese) and source code; newer docs may have removed old flags.

## Current version

<ReleaseVersion />

The file links on the [Download & Install](/en/docs/download), [Server deployment](/en/docs/server/deploy)
and [Language SDKs](/en/docs/sdk/languages) pages are generated from the latest release:

- **On publish**: after the Release Build uploads all assets, the docs workflow is triggered automatically
  and the asset list is regenerated;
- **Manually**: run "文档同步 (Deploy Website)" in Actions, optionally pinning a tag (empty = latest);
- **On website changes**: any push to `website/**` on `master` regenerates them too.

Links are pinned to the version present at build time; you can also use
`https://github.com/lishuangquan1987/aly/releases/latest/download/<asset>` to always fetch the newest file.

- Docs source: [website/docs](https://github.com/lishuangquan1987/aly/tree/master/website/docs)
- English docs: [website/en/docs](https://github.com/lishuangquan1987/aly/tree/master/website/en/docs)

::: tip Align versions before upgrading
The server, publisher and client can be upgraded independently, but a client only accepts a **strictly higher** server version. Read the release notes for the target version before upgrading.
:::