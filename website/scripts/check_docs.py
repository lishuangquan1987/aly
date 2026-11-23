#!/usr/bin/env python3
"""Docs drift check / 文档防漂移核对

Extracts the live inventory of the two Go binaries and cross-checks it against
the website documentation, so a command / flag / API route that disappears
from the docs fails the site build:

  - `aly-publish docs`      -> every visible CLI command and flag (cobra tree)
  - `server -print-routes`  -> every registered route (METHOD PATH)

Usage (from the repository root or anywhere):
  python3 website/scripts/check_docs.py

Exit codes: 0 = no drift, 1 = items missing from the docs (reported),
2 = could not build/run the binaries.

Intentionally undocumented items go to website/scripts/docs-whitelist.txt,
one key per line:  cmd <path-without-aly-publish>  |  flag --<name>  |
route <METHOD> <path>
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WHITELIST = Path(__file__).resolve().parent / "docs-whitelist.txt"

# cobra's own helpers are not part of the product surface.
BUILTIN_PREFIXES = ("completion", "help")


def run_go(module_dir: str, args: list[str]) -> str:
    cmd = ["go", "-C", str(ROOT / module_dir), "run", ".", *args]
    proc = subprocess.run(cmd, capture_output=True)
    if proc.returncode != 0:
        err = (proc.stderr or proc.stdout or b"").decode("utf-8", "replace")
        print(f"failed to run {' '.join(cmd)}\n{err}", file=sys.stderr)
        raise SystemExit(2)
    return proc.stdout.decode("utf-8", "replace")


def load_docs_corpus() -> str:
    parts: list[str] = []
    for pattern in ("website/docs/**/*.md", "website/en/docs/**/*.md",
                    "website/intro.md", "website/en/intro.md"):
        parts.extend(p.read_text(encoding="utf-8", errors="replace")
                     for p in sorted(ROOT.glob(pattern)))
    return "\n".join(parts)


def load_whitelist() -> set[str]:
    if not WHITELIST.exists():
        return set()
    lines = WHITELIST.read_text(encoding="utf-8").splitlines()
    return {line.strip() for line in lines if line.strip() and not line.startswith("#")}


def main() -> int:
    corpus = load_docs_corpus()
    whitelist = load_whitelist()

    missing: list[str] = []

    # ---- CLI commands & flags ----
    for line in run_go("publish/publish-cli", ["docs"]).splitlines():
        if line.startswith("CMD "):
            path = line[4:].strip()
            leaf = path.removeprefix("aly-publish").strip()
            if not leaf or leaf.split()[0] in BUILTIN_PREFIXES:
                continue
            key = f"cmd {leaf}"
            if leaf not in corpus and key not in whitelist:
                missing.append(key)
        elif line.startswith("FLAG "):
            rest = line[5:].strip()
            cmd_path, _, flag_name = rest.rpartition(" --")
            if cmd_path.removeprefix("aly-publish").strip().split()[:1] in ([],) or \
               (cmd_path.removeprefix("aly-publish").strip().split() or [""])[0] in BUILTIN_PREFIXES:
                continue
            key = f"flag --{flag_name}"
            if f"--{flag_name}" not in corpus and key not in whitelist:
                missing.append(key)

    # ---- server routes ----
    for line in run_go("server", ["-print-routes"]).splitlines():
        if not line.strip():
            continue
        method, _, path = line.strip().partition(" ")
        key = f"route {method} {path}"
        if path not in corpus and key not in whitelist:
            missing.append(key)

    if missing:
        print(f"docs drift: {len(missing)} item(s) missing from website docs:\n")
        for item in sorted(missing):
            print(f"  {item}")
        print(f"\nDocument them or add the key to {WHITELIST.relative_to(ROOT)}")
        return 1

    print("docs check ok: every CLI command/flag and server route is documented.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
