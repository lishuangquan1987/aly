from __future__ import annotations

import json
import os
import queue
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from typing import Callable, Dict, Optional

from . import models_gen as m
from .protocol import Response, ng, ok

DEFAULT_SHORT_TIMEOUT = 30.0
DEFAULT_DOWNLOAD_STALL_TIMEOUT = 1800.0
ROLLBACK_TIMEOUT = 600.0

_CREATE_NO_WINDOW = 0x08000000
_BACKGROUND_PROCS: list[subprocess.Popen] = []


def _track_background(proc: subprocess.Popen) -> None:
    """保留仍在运行的子进程引用（模块级，避免 API 临时对象回收时告警）。"""
    _BACKGROUND_PROCS[:] = [p for p in _BACKGROUND_PROCS if p.poll() is None]
    if proc.poll() is None:
        _BACKGROUND_PROCS.append(proc)


@dataclass
class CloseOptions:
    """apply_update / rollback 的进程关闭参数。"""

    must_close_process_names: Optional[str] = None
    close_timeout_seconds: int = 30


@dataclass
class CleanOptions:
    keep: Optional[int] = None
    all: bool = False


@dataclass
class DownloadProgressInfo:
    file: str
    ratio: float
    status: str
    index: int
    total: int
    downloaded_bytes: int = 0
    total_bytes: int = 0
    speed_bps: int = 0


@dataclass
class _RunResult:
    stdout: bytes = b""
    stderr: bytes = b""
    code: int = 0
    timed_out: bool = False
    canceled: bool = False
    start_error: Optional[str] = None


def default_path() -> str:
    name = "aly-client.exe" if os.name == "nt" else "aly-client"
    base = os.path.dirname(os.path.abspath(sys.executable))
    return os.path.normpath(os.path.join(base, "..", "UpdateFolder", name))


def _decode(raw: bytes) -> str:
    return raw.decode("utf-8", errors="replace")


def _popen_kwargs() -> Dict[str, object]:
    kwargs: Dict[str, object] = {}
    if os.name == "nt":
        kwargs["creationflags"] = _CREATE_NO_WINDOW
    return kwargs


class API:
    """封装一个 aly-client 可执行文件。"""

    def __init__(self, path: Optional[str] = None, env: Optional[Dict[str, str]] = None) -> None:
        self.path = os.path.abspath(path or default_path())
        self.env = env

    def _child_env(self) -> Optional[Dict[str, str]]:
        if not self.env:
            return None
        merged = os.environ.copy()
        merged.update(self.env)
        return merged

    def _popen(self, args, *, stdout, stderr, detached: bool = False):
        kwargs = _popen_kwargs()
        if detached and os.name != "nt":
            kwargs["start_new_session"] = True
        return subprocess.Popen(
            [self.path, *args],
            cwd=os.path.dirname(self.path),
            env=self._child_env(),
            stdin=subprocess.DEVNULL,
            stdout=stdout,
            stderr=stderr,
            **kwargs,
        )

    def _run_collect(
        self,
        args,
        timeout: float,
        cancel_event: Optional[threading.Event] = None,
    ) -> _RunResult:
        try:
            proc = self._popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        except OSError as exc:
            return _RunResult(start_error=str(exc))

        started = time.monotonic()
        timed_out = False
        canceled = False
        while proc.poll() is None:
            if cancel_event is not None and cancel_event.is_set():
                canceled = True
                self._kill(proc)
                break
            if time.monotonic() - started > timeout:
                timed_out = True
                self._kill(proc)
                break
            time.sleep(0.02)

        out, err = proc.communicate()
        code = proc.returncode if proc.returncode is not None else 0
        return _RunResult(out, err, code, timed_out, canceled)

    @staticmethod
    def _kill(proc: subprocess.Popen) -> None:
        try:
            proc.kill()
        except OSError:
            pass

    @staticmethod
    def _reap(proc: subprocess.Popen) -> None:
        try:
            proc.wait(timeout=5)
        except (subprocess.TimeoutExpired, OSError):
            pass

    def _track(self, proc: subprocess.Popen) -> None:
        _track_background(proc)

    @staticmethod
    def _parse(result: _RunResult, timeout: float, model=None) -> Response:
        if result.start_error:
            return ng(result.start_error)
        if result.canceled:
            return ng("aly-client call canceled")
        if result.timed_out:
            return ng(f"aly-client timed out after {int(timeout)}s and was killed")

        text = _decode(result.stdout).strip()
        if not text:
            err_text = _decode(result.stderr).strip()
            if result.code != 0:
                return ng(err_text or f"aly-client exited with code {result.code} (may require admin)")
            return ng("aly-client returned no output (may require admin)")

        try:
            payload = json.loads(text)
        except json.JSONDecodeError as exc:
            return ng(str(exc))

        data = payload.get("data")
        typed = model.from_dict(data) if (data is not None and model is not None) else None
        return Response(
            is_success=bool(payload.get("isSuccess", False)),
            error_msg=str(payload.get("errorMsg", "")),
            data=typed,
        )

    def _short(self, command: str, timeout: float, model=None, cancel_event=None) -> Response:
        result = self._run_collect([command], timeout, cancel_event)
        return self._parse(result, timeout, model)

    def check_update(
        self, timeout: float = DEFAULT_SHORT_TIMEOUT, cancel_event=None
    ) -> Response[m.CheckUpdateData]:
        return self._short("check_update", timeout, m.CheckUpdateData, cancel_event)

    def check_self_update(
        self, timeout: float = DEFAULT_SHORT_TIMEOUT, cancel_event=None
    ) -> Response[m.CheckSelfUpdateData]:
        return self._short("check_self_update", timeout, m.CheckSelfUpdateData, cancel_event)

    def self_update(
        self, timeout: float = DEFAULT_SHORT_TIMEOUT, cancel_event=None
    ) -> Response[m.SelfUpdateData]:
        return self._short("self_update", timeout, m.SelfUpdateData, cancel_event)

    def list_rollback_versions(
        self, timeout: float = DEFAULT_SHORT_TIMEOUT, cancel_event=None
    ) -> Response[m.RollbackListData]:
        return self._short("list_rollback_versions", timeout, m.RollbackListData, cancel_event)

    def check_diff(
        self, timeout: float = DEFAULT_SHORT_TIMEOUT, cancel_event=None
    ) -> Response[m.CheckDiffData]:
        return self._short("check_diff", timeout, m.CheckDiffData, cancel_event)

    def clean(
        self,
        options: Optional[CleanOptions] = None,
        timeout: float = DEFAULT_SHORT_TIMEOUT,
        cancel_event=None,
    ) -> Response[m.CleanData]:
        opts = options or CleanOptions()
        args = ["clean"]
        if opts.all:
            args.append("--all")
        elif opts.keep is not None and opts.keep > 0:
            args.extend(["--keep", str(opts.keep)])
        result = self._run_collect(args, timeout, cancel_event)
        return self._parse(result, timeout, m.CleanData)

    def rollback(
        self,
        version: str,
        options: Optional[CloseOptions] = None,
        cancel_event=None,
    ) -> Response[m.RollbackData]:
        opts = options or CloseOptions()
        args = ["rollback", "--version", version]
        _append_close_args(args, opts)
        result = self._run_collect(args, ROLLBACK_TIMEOUT, cancel_event)
        return self._parse(result, ROLLBACK_TIMEOUT, m.RollbackData)

    def download_update(
        self,
        on_progress: Optional[Callable[[DownloadProgressInfo], None]] = None,
        stall_timeout: float = DEFAULT_DOWNLOAD_STALL_TIMEOUT,
        cancel_event: Optional[threading.Event] = None,
    ) -> Response[None]:
        try:
            proc = self._popen(
                ["download_update"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
        except OSError as exc:
            return ng(str(exc))

        lines: "queue.Queue[Optional[bytes]]" = queue.Queue()
        stderr_chunks = []

        def read_stdout() -> None:
            try:
                for raw in iter(proc.stdout.readline, b""):
                    lines.put(raw)
            finally:
                try:
                    proc.stdout.close()
                except OSError:
                    pass
                lines.put(None)

        def read_stderr() -> None:
            try:
                for raw in iter(proc.stderr.readline, b""):
                    stderr_chunks.append(raw)
            except OSError:
                pass
            finally:
                try:
                    proc.stderr.close()
                except OSError:
                    pass

        threading.Thread(target=read_stdout, daemon=True).start()
        threading.Thread(target=read_stderr, daemon=True).start()

        last_output = time.monotonic()
        while True:
            if cancel_event is not None and cancel_event.is_set():
                self._kill(proc)
                self._reap(proc)
                return ng("download canceled")
            try:
                raw = lines.get(timeout=0.2)
            except queue.Empty:
                if time.monotonic() - last_output > stall_timeout:
                    self._kill(proc)
                    self._reap(proc)
                    return ng(
                        f"download stalled: no output for {int(stall_timeout)}s, process killed"
                    )
                continue
            if raw is None:
                self._reap(proc)
                err_text = _decode(b"".join(stderr_chunks)).strip()
                return ng(err_text or "download interrupted (aly-client exited before finish line)")

            last_output = time.monotonic()
            line = _decode(raw).strip()
            if not line:
                continue
            try:
                payload = json.loads(line)
            except json.JSONDecodeError as exc:
                self._kill(proc)
                self._reap(proc)
                return ng(f"invalid progress line: {exc}")

            if not payload.get("isSuccess", False):
                self._kill(proc)
                self._reap(proc)
                return ng(str(payload.get("errorMsg") or "download failed"))

            data = payload.get("data")
            if data is None:
                self._track(proc)
                return ok(None)

            status = str(data.get("status", ""))
            total = int(data.get("total", 0))
            if status == "DONE" and total > 0 and on_progress is not None:
                info = DownloadProgressInfo(
                    file=str(data.get("file", "")),
                    ratio=int(data.get("index", 0)) / total,
                    status=status,
                    index=int(data.get("index", 0)),
                    total=total,
                    downloaded_bytes=int(data.get("downloaded_bytes", 0)),
                    total_bytes=int(data.get("total_bytes", 0)),
                    speed_bps=int(data.get("speed_bps", 0)),
                )
                try:
                    on_progress(info)
                except Exception:
                    pass

    def apply_update(self, options: Optional[CloseOptions] = None) -> Response[None]:
        opts = options or CloseOptions()
        args = ["apply_update"]
        _append_close_args(args, opts)
        try:
            proc = self._popen(args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, detached=True)
        except OSError as exc:
            return ng(str(exc))
        self._track(proc)
        return ok(None)


def _append_close_args(args, options: CloseOptions) -> None:
    names = (options.must_close_process_names or "").strip()
    if names:
        args.extend(["--must-close-process-name", names])
    if options.close_timeout_seconds and options.close_timeout_seconds != 30:
        args.extend(["--close-timeout", str(options.close_timeout_seconds)])