#!/usr/bin/env python3
"""Capture real Aly UI screenshots for the documentation site.

This script is Windows-only and is a development tool: it is not part of the
VitePress build. It creates a throwaway demo package, starts the real server,
publishes two versions with the real CLI, then captures the real publish GUI and
the real end-user update window.

Requirements:
  - Windows interactive desktop
  - .NET 8 runtime (for AlyPublish.exe)
  - Python 3.9+ with Pillow
  - the repository binaries built (dist-test/ and the publish-gui-csharp Debug output)

Usage:
  python website/scripts/capture_screenshots.py
  python website/scripts/capture_screenshots.py --keep

The script temporarily replaces %LOCALAPPDATA%\\AlyPublish\\config.json while the
publish GUI is open, and restores it afterwards. It refuses to run if AlyPublish
or aly-client-ui is already running.
"""

from __future__ import annotations

import argparse
import ctypes
import json
import os
import shutil
import socket
import sqlite3
import subprocess
import sys
import time
import traceback
from ctypes import wintypes
from pathlib import Path
from typing import Optional

from PIL import Image, ImageGrab

ROOT = Path(__file__).resolve().parents[2]
WEBSITE = ROOT / "website"
OUT_DIR = WEBSITE / "public" / "images"
TMP_DIR = Path(os.environ.get("ALY_SCREENSHOT_TMP", r"C:\AlyDemo"))

DIST_BIN = ROOT / "dist-test"
PUBLISH_GUI_BIN = ROOT / "publish" / "publish-gui-csharp" / "src" / "AlyPublish" / "bin" / "Debug" / "net8.0"
CLIENT_UI_EXE = ROOT / "client" / "aly-client-ui" / "aly-client-winui" / "aly-client-ui.exe"

SERVER_PORT = 29100
SERVER_URL = f"http://127.0.0.1:{SERVER_PORT}"
PROJECT_NAME = "demo-app"
PROJECT_TITLE = "演示应用"
PROJECT_TITLE_EN = "Demo App"

UI_TITLE_ZH = "AlyPublish - 版本发布工具"
UI_TITLE_EN = "AlyPublish - Release Publisher"

# ── Win32 helpers ─────────────────────────────────────────────────────

user32 = ctypes.windll.user32
dwmapi = ctypes.windll.dwmapi
gdi32 = ctypes.windll.gdi32

DWMWA_EXTENDED_FRAME_BOUNDS = 9
SW_RESTORE = 9
HWND_TOP = 0
SWP_SHOWWINDOW = 0x0040
SWP_NOSIZE = 0x0001
SWP_NOMOVE = 0x0002
WM_CLOSE = 0x0010
HWND_TOPMOST = ctypes.c_void_p(-1)
HWND_NOTOPMOST = ctypes.c_void_p(-2)


class RECT(ctypes.Structure):
    _fields_ = [
        ("left", ctypes.c_long),
        ("top", ctypes.c_long),
        ("right", ctypes.c_long),
        ("bottom", ctypes.c_long),
    ]


class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [
        ("biSize", wintypes.DWORD),
        ("biWidth", wintypes.LONG),
        ("biHeight", wintypes.LONG),
        ("biPlanes", wintypes.WORD),
        ("biBitCount", wintypes.WORD),
        ("biCompression", wintypes.DWORD),
        ("biSizeImage", wintypes.DWORD),
        ("biXPelsPerMeter", wintypes.LONG),
        ("biYPelsPerMeter", wintypes.LONG),
        ("biClrUsed", wintypes.DWORD),
        ("biClrImportant", wintypes.DWORD),
    ]


class BITMAPINFO(ctypes.Structure):
    _fields_ = [
        ("bmiHeader", BITMAPINFOHEADER),
        ("bmiColors", wintypes.DWORD * 3),
    ]


def enable_dpi_awareness() -> None:
    """Make ImageGrab coordinates physical pixels on scaled displays."""
    try:
        # PROCESS_PER_MONITOR_DPI_AWARE
        ctypes.windll.shcore.SetProcessDpiAwareness(2)
    except Exception:
        try:
            user32.SetProcessDPIAware()
        except Exception:
            pass


def window_bounds(hwnd: int) -> tuple[int, int, int, int]:
    rect = RECT()
    try:
        dwmapi.DwmGetWindowAttribute(
            wintypes.HWND(hwnd),
            ctypes.c_uint(DWMWA_EXTENDED_FRAME_BOUNDS),
            ctypes.byref(rect),
            ctypes.sizeof(rect),
        )
    except Exception:
        rect = RECT()
    if rect.right <= rect.left or rect.bottom <= rect.top:
        rect = RECT()
        if not user32.GetWindowRect(wintypes.HWND(hwnd), ctypes.byref(rect)):
            raise RuntimeError("GetWindowRect failed")
    return rect.left, rect.top, rect.right, rect.bottom


def enumerate_windows() -> list[tuple[int, int, str]]:
    result: list[tuple[int, int, str]] = []
    EnumProc = ctypes.WINFUNCTYPE(ctypes.c_bool, wintypes.HWND, wintypes.LPARAM)

    def callback(hwnd, _lparam):
        if not user32.IsWindowVisible(hwnd):
            return True
        length = user32.GetWindowTextLengthW(hwnd)
        if length <= 0:
            return True
        buf = ctypes.create_unicode_buffer(length + 1)
        user32.GetWindowTextW(hwnd, buf, length + 1)
        pid = wintypes.DWORD()
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
        result.append((int(hwnd), int(pid.value), buf.value))
        return True

    user32.EnumWindows(EnumProc(callback), 0)
    return result


def find_window(pid: int, title_part: str = "", timeout: float = 25.0) -> int:
    deadline = time.time() + timeout
    while time.time() < deadline:
        for hwnd, win_pid, title in enumerate_windows():
            if win_pid != pid:
                continue
            if title_part and title_part not in title:
                continue
            return hwnd
        time.sleep(0.25)
    raise TimeoutError(f"window not found for pid={pid}, title contains {title_part!r}")


def bring_to_front(hwnd: int, x: int = 60, y: int = 60, width: Optional[int] = None, height: Optional[int] = None) -> None:
    user32.ShowWindow(wintypes.HWND(hwnd), SW_RESTORE)
    user32.SetForegroundWindow(wintypes.HWND(hwnd))
    flags = SWP_SHOWWINDOW
    if width is None or height is None:
        flags |= SWP_NOSIZE
    user32.SetWindowPos(
        wintypes.HWND(hwnd),
        wintypes.HWND(HWND_TOP),
        x,
        y,
        width or 0,
        height or 0,
        flags,
    )
    user32.SetCursorPos(8, 8)
    time.sleep(0.8)


def close_window(hwnd: int, pid: int) -> None:
    user32.PostMessageW(wintypes.HWND(hwnd), WM_CLOSE, 0, 0)
    deadline = time.time() + 8
    while time.time() < deadline:
        if not user32.IsWindow(wintypes.HWND(hwnd)):
            return
        time.sleep(0.2)
    kill_pid(pid)


def set_topmost(hwnd: int, enabled: bool) -> None:
    user32.SetWindowPos(
        wintypes.HWND(hwnd),
        HWND_TOPMOST if enabled else HWND_NOTOPMOST,
        0,
        0,
        0,
        0,
        SWP_NOMOVE | SWP_NOSIZE | SWP_SHOWWINDOW,
    )
    time.sleep(0.6)


def grab_window(hwnd: int) -> Image.Image:
    """Capture a window by handle, not by screen region.

    ImageGrab captures whatever is physically on top at those coordinates. If
    another always-on-top window overlaps the target, the screenshot silently
    contains the wrong application. PrintWindow asks the target window to paint
    itself into a memory DC, so occlusion cannot leak into the docs image.
    """
    rect = RECT()
    if not user32.GetWindowRect(wintypes.HWND(hwnd), ctypes.byref(rect)):
        raise RuntimeError("GetWindowRect failed")
    width = rect.right - rect.left
    height = rect.bottom - rect.top
    if width <= 0 or height <= 0:
        raise RuntimeError(f"invalid window size {width}x{height}")

    window_dc = user32.GetWindowDC(wintypes.HWND(hwnd))
    memory_dc = gdi32.CreateCompatibleDC(window_dc)
    bitmap = gdi32.CreateCompatibleBitmap(window_dc, width, height)
    previous = gdi32.SelectObject(memory_dc, bitmap)
    try:
        if not user32.PrintWindow(wintypes.HWND(hwnd), memory_dc, 2):
            user32.PrintWindow(wintypes.HWND(hwnd), memory_dc, 0)

        info = BITMAPINFO()
        info.bmiHeader.biSize = ctypes.sizeof(BITMAPINFOHEADER)
        info.bmiHeader.biWidth = width
        info.bmiHeader.biHeight = -height
        info.bmiHeader.biPlanes = 1
        info.bmiHeader.biBitCount = 32
        info.bmiHeader.biCompression = 0  # BI_RGB

        buffer = ctypes.create_string_buffer(width * height * 4)
        if not gdi32.GetDIBits(
            memory_dc, bitmap, 0, height, buffer, ctypes.byref(info), 0
        ):
            raise RuntimeError("GetDIBits failed")

        image = Image.frombuffer(
            "RGB", (width, height), buffer, "raw", "BGRX", 0, 1
        ).copy()
        if image.getextrema() == ((0, 0), (0, 0), (0, 0)):
            raise RuntimeError("PrintWindow produced an all-black image")
        return image
    finally:
        gdi32.SelectObject(memory_dc, previous)
        gdi32.DeleteObject(bitmap)
        gdi32.DeleteDC(memory_dc)
        user32.ReleaseDC(wintypes.HWND(hwnd), window_dc)


def capture_window(hwnd: int, dest: Path, max_width: int = 1600) -> None:
    try:
        image = grab_window(hwnd)
    except Exception as exc:
        print(f"  PrintWindow failed ({exc}); falling back to screen grab")
        user32.BringWindowToTop(wintypes.HWND(hwnd))
        user32.SetForegroundWindow(wintypes.HWND(hwnd))
        time.sleep(0.5)
        left, top, right, bottom = window_bounds(hwnd)
        image = ImageGrab.grab(bbox=(left, top, right, bottom), all_screens=True)
    if image.width > max_width:
        height = round(image.height * max_width / image.width)
        image = image.resize((max_width, height), Image.LANCZOS)
    dest.parent.mkdir(parents=True, exist_ok=True)
    image.convert("RGB").save(dest, "WEBP", quality=82, method=6)
    print(f"  saved {dest.relative_to(ROOT)} ({image.width}x{image.height})")


def kill_pid(pid: int) -> None:
    subprocess.run(
        ["taskkill", "/F", "/PID", str(pid)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def kill_image(image_name: str) -> None:
    subprocess.run(
        ["taskkill", "/F", "/IM", image_name],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def image_running(image_name: str) -> bool:
    proc = subprocess.run(
        ["tasklist", "/FI", f"IMAGENAME eq {image_name}", "/FO", "CSV", "/NH"],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    return image_name.lower() in (proc.stdout or "").lower()


# ── Demo data & server ───────────────────────────────────────────────


def run(cmd: list[str], cwd: Optional[Path] = None, check: bool = True) -> subprocess.CompletedProcess:
    proc = subprocess.run(
        cmd,
        cwd=str(cwd) if cwd else None,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        creationflags=subprocess.CREATE_NO_WINDOW,
    )
    if check and proc.returncode != 0:
        raise RuntimeError(
            f"command failed ({proc.returncode}): {' '.join(cmd)}\nstdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return proc


def json_lines(output: str) -> list[dict]:
    payloads: list[dict] = []
    for line in (output or "").splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            payloads.append(value)
    return payloads


def publish(*args: str, cwd: Path, expect_json: bool = True) -> Optional[dict]:
    cmd = [str(DIST_BIN / "aly-publish.exe"), *args]
    proc = run(cmd, cwd=cwd)
    if not expect_json:
        return None
    payloads = json_lines(proc.stdout)
    if not payloads:
        raise RuntimeError(f"no JSON output from: {' '.join(cmd)}\n{proc.stdout}\n{proc.stderr}")
    failures = [p for p in payloads if not p.get("isSuccess", False)]
    if failures:
        message = failures[0].get("errorMsg") or failures[0]
        raise RuntimeError(f"publish command failed: {' '.join(args)}\n{message}\n{proc.stdout}")
    return payloads[-1]


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def write_bytes(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def seed_files(dist: Path) -> None:
    fake_main = DIST_BIN / "fake_main.exe"
    if fake_main.exists():
        shutil.copy2(fake_main, dist / "DemoApp.exe")
    else:
        write_bytes(dist / "DemoApp.exe", b"MZ" + b"0" * 4096)

    write_bytes(dist / "DemoApp.dll", b"Aly demo assembly\n" + b"d" * 90000)
    write_bytes(dist / "assets" / "theme.pak", b"Aly theme pack\n" + b"t" * 140000)
    write_text(
        dist / "config" / "default.json",
        json.dumps(
            {
                "language": "zh-CN",
                "server": "https://demo.invalid/api",
                "features": {"offline": True, "telemetry": False},
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
    )
    write_bytes(dist / "plugins" / "report.dll", b"report plugin\n" + b"r" * 42000)
    write_bytes(dist / "plugins" / "export.dll", b"export plugin\n" + b"e" * 36000)
    write_text(dist / "resources" / "lang-zh.json", '{"hello":"你好","settings":"设置"}\n')
    write_text(dist / "resources" / "lang-en.json", '{"hello":"Hello","settings":"Settings"}\n')
    write_text(dist / "docs" / "readme.txt", "Aly demo application\n\nThis folder simulates a build output.\n")
    write_bytes(dist / "runtime" / "native.dll", b"native runtime\n" + b"n" * 65000)


def seed_changes(dist: Path, stage_three: bool) -> None:
    """Create modified + new files, optionally staging a subset."""
    for rel in ("DemoApp.dll", "assets/theme.pak", "plugins/export.dll"):
        path = dist / rel
        with path.open("ab") as fh:
            fh.write(b"change-set " + str(time.time_ns()).encode())
    unique = str(time.time_ns()).encode()
    write_bytes(dist / "plugins" / "license.dll", b"license plugin\n" + unique + b"\n" + b"l" * 28000)
    write_text(
        dist / "docs" / "changelog-1.0.3.txt",
        f"1.0.3\n- faster downloads ({unique.decode()})\n- fixed login timeout\n",
    )

    if stage_three:
        publish("add", "DemoApp.dll", "assets/theme.pak", "plugins/license.dll", "--json", cwd=dist)


def start_server(server_dir: Path) -> tuple[subprocess.Popen, object]:
    server_dir.mkdir(parents=True, exist_ok=True)
    exe = server_dir / "aly-server.exe"
    shutil.copy2(DIST_BIN / "server.exe", exe)
    log_path = server_dir / "server.log"
    log = log_path.open("w", encoding="utf-8")
    proc = subprocess.Popen(
        [str(exe), "-p", str(SERVER_PORT), "-db", str(server_dir / "aly.db")],
        cwd=str(server_dir),
        stdout=log,
        stderr=subprocess.STDOUT,
        creationflags=subprocess.CREATE_NO_WINDOW,
    )
    deadline = time.time() + 20
    while time.time() < deadline:
        if proc.poll() is not None:
            log.close()
            raise RuntimeError(f"server exited early; see {log_path}")
        try:
            with socket.create_connection(("127.0.0.1", SERVER_PORT), timeout=0.3):
                log.flush()
                return proc, log
        except OSError:
            time.sleep(0.25)
    proc.terminate()
    log.close()
    raise TimeoutError("server did not start listening")


def localize_initial_log(db_path: Path, project_name: str, text: str) -> None:
    """Rewrite the server-generated initial changelog for the English demo project."""
    connection = sqlite3.connect(db_path)
    try:
        row = connection.execute(
            "SELECT id FROM projects WHERE name = ?", (project_name,)
        ).fetchone()
        if row is None:
            return
        connection.execute(
            "UPDATE project_change_logs SET logs = ? WHERE project_id = ? AND version = 'V1.0.0'",
            (json.dumps([text], ensure_ascii=False), row[0]),
        )
        connection.commit()
    finally:
        connection.close()


def write_shared(app_folder: Path, project_name: str) -> None:
    write_text(
        app_folder / ".updator" / "shared.json",
        json.dumps(
            {
                "server_url": SERVER_URL,
                "project_name": project_name,
                "ignore_folders": [],
                "ignore_files": [],
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
    )


def setup_demo(tmp: Path, server: subprocess.Popen) -> tuple[Path, Path, Path, Path]:
    """Return (dist, update_folder, app_folder) for the demo package."""
    if server.poll() is not None:
        raise RuntimeError("server is not running")
    dist = tmp / "demo" / "dist"
    dist.mkdir(parents=True, exist_ok=True)
    seed_files(dist)

    publish(
        "project",
        "create",
        "--name",
        PROJECT_NAME,
        "--title",
        PROJECT_TITLE,
        "--server",
        SERVER_URL,
        "--json",
        cwd=tmp,
    )
    publish(
        "config",
        "init",
        "--project",
        PROJECT_NAME,
        "--server",
        SERVER_URL,
        "--ignore-folders",
        "logs,temp",
        "--ignore-files",
        "*.log,*.tmp",
        "--json",
        cwd=dist,
    )
    publish("status", "--json", cwd=dist)
    publish("add", "--all", "--json", cwd=dist)
    publish(
        "push",
        "--version",
        "V1.0.1",
        "--message",
        "首个可发布版本",
        "--message",
        "补齐离线资源包",
        "--json",
        cwd=dist,
    )
    publish("reset", "--all", "--json", cwd=dist)

    seed_changes(dist, stage_three=False)
    publish("add", "--all", "--json", cwd=dist)
    publish(
        "push",
        "--version",
        "V1.0.2",
        "--message",
        "修复登录超时",
        "--message",
        "优化大文件下载进度",
        "--json",
        cwd=dist,
    )
    publish("reset", "--all", "--json", cwd=dist)

    # Fresh changes for the screenshot: 3 staged, 2 unstaged.
    seed_changes(dist, stage_three=True)
    publish("status", "--json", cwd=dist)

    package = tmp / "demo" / "package"
    app_folder = package / "ApplicationFolder"
    update_folder = package / "UpdateFolder"
    app_folder.mkdir(parents=True, exist_ok=True)
    update_folder.mkdir(parents=True, exist_ok=True)
    shutil.copy2(DIST_BIN / "fake_main.exe", app_folder / "DemoApp.exe")
    shutil.copy2(DIST_BIN / "aly-client.exe", update_folder / "aly-client.exe")
    shutil.copy2(CLIENT_UI_EXE, update_folder / "aly-client-ui.exe")
    write_shared(app_folder, PROJECT_NAME)

    # A second project with English release notes for the English screenshot.
    en_dist = tmp / "demo-en" / "dist"
    en_dist.mkdir(parents=True, exist_ok=True)
    seed_files(en_dist)
    publish(
        "project",
        "create",
        "--name",
        "demo-app-en",
        "--title",
        "Demo App",
        "--server",
        SERVER_URL,
        "--json",
        cwd=tmp,
    )
    publish(
        "config",
        "init",
        "--project",
        "demo-app-en",
        "--server",
        SERVER_URL,
        "--json",
        cwd=en_dist,
    )
    publish("add", "--all", "--json", cwd=en_dist)
    publish(
        "push",
        "--version",
        "V1.0.1",
        "--message",
        "First releasable build",
        "--message",
        "Offline resources included",
        "--json",
        cwd=en_dist,
    )
    publish("reset", "--all", "--json", cwd=en_dist)
    seed_changes(en_dist, stage_three=False)
    publish("add", "--all", "--json", cwd=en_dist)
    publish(
        "push",
        "--version",
        "V1.0.2",
        "--message",
        "Fix login timeout",
        "--message",
        "Faster large-file downloads",
        "--json",
        cwd=en_dist,
    )
    publish("reset", "--all", "--json", cwd=en_dist)
    seed_changes(en_dist, stage_three=True)
    publish("status", "--json", cwd=en_dist)
    localize_initial_log(tmp / "server" / "aly.db", "demo-app-en", "Initial version")
    write_text(
        update_folder / "client.json",
        json.dumps(
            {
                "main_exe_relative_path": "../ApplicationFolder/DemoApp.exe",
                "must_close_process_name": ["DemoApp"],
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
    )
    write_text(
        update_folder / "version.json",
        json.dumps({"version": "1.0.1", "version_status": "applied"}, indent=2) + "\n",
    )
    return dist, en_dist, update_folder, app_folder


# ── Publish GUI ──────────────────────────────────────────────────────


def publish_gui_config_path() -> Path:
    local = os.environ.get("LOCALAPPDATA", "")
    if not local:
        raise RuntimeError("LOCALAPPDATA is not set")
    return Path(local) / "AlyPublish" / "config.json"


def capture_publish_gui(
    dist: Path,
    tmp: Path,
    language: str,
    title: str,
    dest_name: str,
    display_name: str,
) -> None:
    if image_running("AlyPublish.exe"):
        raise RuntimeError("AlyPublish.exe is already running; close it before capturing")

    config_path = publish_gui_config_path()
    backup = tmp / "AlyPublish.config.backup.json"
    had_config = config_path.exists()
    if had_config:
        shutil.copy2(config_path, backup)
    config_path.parent.mkdir(parents=True, exist_ok=True)
    config_path.write_text(
        json.dumps(
            [
                {
                    "DisplayName": display_name,
                    "ProjectPath": str(dist),
                    "MainExePath": "",
                }
            ],
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    env = os.environ.copy()
    env["LOCALAPPDATA"] = str(tmp / "localappdata")
    (tmp / "localappdata").mkdir(parents=True, exist_ok=True)
    proc = subprocess.Popen(
        [str(PUBLISH_GUI_BIN / "AlyPublish.exe"), "--lang", language],
        cwd=str(PUBLISH_GUI_BIN),
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        hwnd = find_window(proc.pid, title, timeout=30)
        bring_to_front(hwnd, x=50, y=50, width=1440, height=900)
        time.sleep(6)
        set_topmost(hwnd, True)
        time.sleep(0.8)
        capture_window(hwnd, OUT_DIR / dest_name, max_width=1600)
        set_topmost(hwnd, False)
        close_window(hwnd, proc.pid)
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
        if had_config:
            shutil.copy2(backup, config_path)
        else:
            config_path.unlink(missing_ok=True)


# ── Client update window ─────────────────────────────────────────────


def capture_client_ui(
    update_folder: Path,
    app_folder: Path,
    project_name: str,
    language: str,
    title: str,
    dest_name: str,
) -> None:
    write_shared(app_folder, project_name)
    kill_image("aly-client-ui.exe")
    time.sleep(0.5)
    proc = subprocess.Popen(
        [
            str(update_folder / "aly-client-ui.exe"),
            "-mode",
            "check",
            "-lang",
            language,
            "-project-name",
            title,
        ],
        cwd=str(update_folder),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        hwnd = find_window(proc.pid, title, timeout=30)
        # The card shows "checking" first; wait for the update result.
        time.sleep(4.5)
        bring_to_front(hwnd, x=180, y=140)
        set_topmost(hwnd, True)
        time.sleep(0.8)
        capture_window(hwnd, OUT_DIR / dest_name, max_width=1200)
        close_window(hwnd, proc.pid)
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
        time.sleep(0.6)


# ── Entry point ──────────────────────────────────────────────────────


def main() -> int:
    parser = argparse.ArgumentParser(description="Capture Aly UI screenshots")
    parser.add_argument("--keep", action="store_true", help="keep the temporary demo directory")
    args = parser.parse_args()

    required = [
        DIST_BIN / "server.exe",
        DIST_BIN / "aly-publish.exe",
        DIST_BIN / "aly-client.exe",
        DIST_BIN / "fake_main.exe",
        PUBLISH_GUI_BIN / "AlyPublish.exe",
        PUBLISH_GUI_BIN / "aly-publish.exe",
        CLIENT_UI_EXE,
    ]
    missing = [str(p) for p in required if not p.exists()]
    if missing:
        print("Missing binaries:\n  " + "\n  ".join(missing), file=sys.stderr)
        print("Build the server/CLI/client and publish GUI first.", file=sys.stderr)
        return 2

    enable_dpi_awareness()
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    if TMP_DIR.exists():
        shutil.rmtree(TMP_DIR)
    TMP_DIR.mkdir(parents=True, exist_ok=True)

    server = None
    server_log = None
    try:
        print("Preparing demo data and starting the real server...")
        server, server_log = start_server(TMP_DIR / "server")
        dist, en_dist, update_folder, app_folder = setup_demo(TMP_DIR, server)

        print("Capturing publish GUI (zh-CN)...")
        capture_publish_gui(
            dist, TMP_DIR, "zh-CN", UI_TITLE_ZH, "publish-gui-overview-zh.webp", PROJECT_TITLE
        )
        print("Capturing publish GUI (en-US)...")
        capture_publish_gui(
            en_dist, TMP_DIR, "en-US", UI_TITLE_EN, "publish-gui-overview.webp", PROJECT_TITLE_EN
        )
        print("Capturing client update window (zh-CN)...")
        capture_client_ui(
            update_folder,
            app_folder,
            PROJECT_NAME,
            "zh-CN",
            PROJECT_TITLE,
            "client-ui-update-zh.webp",
        )
        print("Capturing client update window (en-US)...")
        capture_client_ui(
            update_folder,
            app_folder,
            "demo-app-en",
            "en-US",
            PROJECT_TITLE_EN,
            "client-ui-update-en.webp",
        )
    except Exception as exc:
        print(f"Capture failed: {exc}", file=sys.stderr)
        traceback.print_exc()
        return 1
    finally:
        kill_image("aly-client-ui.exe")
        if server is not None and server.poll() is None:
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
        if server_log is not None:
            server_log.close()
        if not args.keep:
            shutil.rmtree(TMP_DIR, ignore_errors=True)
        else:
            print(f"kept temporary directory: {TMP_DIR}")

    print("Done. Images written to website/public/images/.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())