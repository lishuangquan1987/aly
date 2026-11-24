#!/usr/bin/env python3
"""Docs structure check / 文档结构与起步页护栏

两个检查，任一失败即退出码 1：

  1. 页面对齐：website/docs/**/*.md 与 website/en/docs/**/*.md 的相对路径集合必须完全一致，
     每个页面必须有 frontmatter title。新增中文页面时必须同时给英文页面，反之亦然。
  2. 起步页零原理：website/docs/start/*.md 与 website/en/docs/start/*.md 不得出现原理类词汇
     （状态机/原子替换/架构、state machine/atomic/architecture），必须给出 ✅ 完成态与至少一个代码块。
     这就是「快速开始只讲怎么用起来」的自动护栏。

Usage:
  python3 website/scripts/check_docs_parity.py
"""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "website" / "docs"
EN_DOCS = ROOT / "website" / "en" / "docs"

# 起步页禁止出现的原理表述：这些内容属于「原理」与「参考」。
BANNED_ZH = ("状态机", "原子替换", "原子改名", "架构")
BANNED_EN = ("state machine", "atomic", "architecture")


def relative_pages(root: Path) -> set[str]:
    return {p.relative_to(root).as_posix() for p in root.rglob("*.md")}


def has_title(path: Path) -> bool:
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    if not lines or lines[0].strip() != "---":
        return False
    for line in lines[1:]:
        if line.strip() == "---":
            return False
        if line.startswith("title:"):
            return True
    return False


def check_start_page(path: Path, banned: tuple[str, ...]) -> list[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
    problems: list[str] = []

    lowered = text.lower()
    hits = [word for word in banned if word.lower() in lowered]
    if hits:
        problems.append(f"principle wording on a getting-started page: {', '.join(hits)}")
    if "✅" not in text:
        problems.append("missing the ✅ done marker")
    if "```" not in text:
        problems.append("missing a fenced code block")
    return problems


def main() -> int:
    problems: list[str] = []

    zh_pages = relative_pages(DOCS)
    en_pages = relative_pages(EN_DOCS)

    only_zh = sorted(zh_pages - en_pages)
    only_en = sorted(en_pages - zh_pages)
    for page in only_zh:
        problems.append(f"only in website/docs: docs/{page} (add en/docs/{page})")
    for page in only_en:
        problems.append(f"only in website/en/docs: en/docs/{page} (add docs/{page})")

    for page in sorted(zh_pages & en_pages):
        for base, label in ((DOCS, "docs"), (EN_DOCS, "en/docs")):
            if not has_title(base / page):
                problems.append(f"missing frontmatter title: {label}/{page}")

    for path in sorted((DOCS / "start").glob("*.md")):
        for problem in check_start_page(path, BANNED_ZH):
            problems.append(f"{path.relative_to(ROOT).as_posix()}: {problem}")
    for path in sorted((EN_DOCS / "start").glob("*.md")):
        for problem in check_start_page(path, BANNED_EN):
            problems.append(f"{path.relative_to(ROOT).as_posix()}: {problem}")

    if problems:
        print(f"docs structure: {len(problems)} problem(s)\n")
        for problem in problems:
            print(f"  {problem}")
        return 1

    print(
        f"docs structure ok: {len(zh_pages)} pages per language, "
        "getting-started pages stay principle-free."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
