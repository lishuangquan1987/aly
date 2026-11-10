from __future__ import annotations

import contextlib
import os
import subprocess
import tempfile
import time
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
_FAKE = None


def fake_client() -> str:
    """构建并缓存 tools/fake-aly-client。"""
    global _FAKE
    if _FAKE is None:
        out_dir = tempfile.mkdtemp(prefix="aly-py-fake-")
        name = "aly-client.exe" if os.name == "nt" else "aly-client"
        out = os.path.join(out_dir, name)
        subprocess.run(
            ["go", "build", "-o", out, "."],
            cwd=str(REPO_ROOT / "tools" / "fake-aly-client"),
            check=True,
        )
        _FAKE = out
    return _FAKE


@contextlib.contextmanager
def scenario(name: str):
    previous = os.environ.get("ALY_FAKE_SCENARIO")
    os.environ["ALY_FAKE_SCENARIO"] = str(
        REPO_ROOT / "spec" / "fixtures" / "scenarios" / f"{name}.json"
    )
    try:
        yield
    finally:
        if previous is None:
            os.environ.pop("ALY_FAKE_SCENARIO", None)
        else:
            os.environ["ALY_FAKE_SCENARIO"] = previous


def wait_for(predicate, timeout: float = 5.0, interval: float = 0.02) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(interval)
    raise AssertionError(f"wait_for timeout after {timeout}s")