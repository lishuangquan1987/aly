# aly-publish Tool

`aly-publish` is the publisher-side CLI (cobra-based) for project config, diff scanning, staging and pushing. The desktop app `aly-publish-gui` (Avalonia) wraps the same capabilities for non-technical publishers.

## Command Tree

```text
aly-publish
├── config            # project config (.updator/ directory)
│   ├── init          #   initialize (--server --project)
│   ├── set           #   set a single option
│   ├── set-array     #   set array options (e.g. ignore rules)
│   ├── get           #   read a single option
│   ├── list          #   list all options
│   └── path          #   show the config file path
├── project           # server-side project lifecycle
│   ├── list          #   list all server projects
│   ├── create        #   create a project
│   ├── update        #   update project config
│   ├── delete        #   delete a project
│   └── info          #   project details
├── status            # diff overview: local vs. server
├── diff              # per-file comparison (added/modified/deleted + hashes)
├── add               # stage changes (--all or specific paths)
├── reset             # unstage
├── staged            # show staged content
├── push              # push staged files and publish a version (--version --message)
├── log               # view version changelog
└── server info       # server system info
```

## Typical Workflow

```bash
# One-time: enter the release directory, initialize
cd ./dist
aly-publish config init --server http://10.0.0.1:2000 --project myapp

# Set ignore rules (shipped with the release; the client honors them when applying too)
aly-publish config set-array ignore_folders logs temp
aly-publish config set-array ignore_files "*.log" "*.tmp"

# Daily release: three steps
aly-publish status           # what changed this time
aly-publish add --all        # stage everything
aly-publish push --version V1.0.2 --message "Fix login timeout"
```

## Key Behaviors

- **Diff scanning**: `status` / `diff` compare against the server's latest manifest using MD5 + SHA256, reporting added / modified / deleted.
- **Chunked upload**: files above the threshold automatically use the `upload_chunk` → `upload_chunk_complete` pipeline with resumable transfers.
- **Ignore rules**: `ignore_folders` / `ignore_files` apply on both ends — scanning (what gets uploaded) and client apply (what gets skipped when copying).
- **Version monotonicity**: the version used by `push` must be strictly greater than the server's latest, otherwise the server refuses — to roll content back, publish a higher version number.
- **Publish GUI**: `publish-gui` shares `.updator/` config and staging with the CLI; the two can be mixed freely.
