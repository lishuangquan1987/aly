from __future__ import annotations

import random
import threading
from typing import Callable, Optional

from .api import API, CloseOptions
from .protocol import Response


class Status:
    """更新生命周期阶段。"""

    NONE = "none"
    DISCOVERED_UPDATE = "discoveredUpdate"
    DOWNLOADING_UPDATE = "downloadingUpdate"
    DOWNLOADED_UPDATE = "downloadedUpdate"
    APPLY_UPDATE = "applyUpdate"


class UpdateClient:
    """
    aly-client 的托管生命周期：自更新 → check → download → apply。

    确认回调在循环线程内同步执行，签名 ``(version, stop_event)``；
    返回即确认。阻塞时应等待 ``stop_event`` 以便 ``stop()`` 能及时退出。
    """

    def __init__(
        self,
        path: Optional[str] = None,
        *,
        api: Optional[API] = None,
        idle_poll_interval: float = 300.0,
        must_close_process_names: Optional[str] = None,
        close_timeout_seconds: int = 30,
    ) -> None:
        self.api = api or API(path)
        self.idle_poll_interval = idle_poll_interval
        self.must_close_process_names = must_close_process_names
        self.close_timeout_seconds = close_timeout_seconds

        self.on_status_change: Optional[Callable[[str, str], None]] = None
        self.on_error: Optional[Callable[[str], None]] = None
        self.confirm_download: Optional[Callable[[str, threading.Event], None]] = None
        self.confirm_apply: Optional[Callable[[str, threading.Event], None]] = None

        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._status = Status.NONE
        self._error = ""

    @property
    def status(self) -> str:
        with self._lock:
            return self._status

    @property
    def last_error(self) -> str:
        with self._lock:
            return self._error

    @property
    def is_running(self) -> bool:
        thread = self._thread
        return thread is not None and thread.is_alive()

    def start(self) -> None:
        """启动后台循环（立即返回）。重复调用无副作用。"""
        with self._lock:
            if self._thread is not None and self._thread.is_alive():
                return
            self._stop.clear()
            self._thread = threading.Thread(target=self._loop, name="aly-update", daemon=True)
            self._thread.start()

    def stop(self, timeout: float = 5.0) -> bool:
        """停止轮询并等待循环退出。返回是否在超时内退出。"""
        self._stop.set()
        thread = self._thread
        if thread is not None:
            thread.join(timeout)
            return not thread.is_alive()
        return True

    def _loop(self) -> None:
        self._try_self_update()
        while not self._stop.is_set():
            check = self.api.check_update(cancel_event=self._stop)
            if self._stop.is_set():
                return
            if not check.is_success:
                self._report_error(check.error_msg)
                if self._wait(5.0):
                    return
                continue
            self._clear_error()

            data = check.data
            if data is None:
                self._report_error("check_update returned no data")
                if self._wait(5.0):
                    return
                continue

            if not data.has_update or not data.need_download_update:
                if self.status != Status.NONE:
                    self._set_status(Status.NONE, "Update status cleared")
                if self._wait(self._idle_delay()):
                    return
                continue

            version = data.new_version or ""
            message = f"Found new version {version}".strip()
            self._set_status(Status.DISCOVERED_UPDATE, message)
            if not data.force_update:
                self._call_confirm(self.confirm_download, version)
                if self._stop.is_set():
                    return

            self._set_status(Status.DOWNLOADING_UPDATE, "Downloading update...")
            downloaded = self.api.download_update(
                on_progress=lambda info: self._set_status(
                    Status.DOWNLOADING_UPDATE,
                    f"Downloading {info.file}... {int(info.ratio * 100)}%",
                ),
                cancel_event=self._stop,
            )
            if self._stop.is_set():
                return
            if not downloaded.is_success:
                if "already at latest version" in downloaded.error_msg:
                    self._set_status(Status.NONE, "Update status cleared")
                    if self._wait(self._idle_delay()):
                        return
                    continue
                self._report_error(downloaded.error_msg)
                if self._wait(1.0):
                    return
                continue

            self._set_status(Status.DOWNLOADED_UPDATE, "Update downloaded")
            if not data.force_update:
                self._call_confirm(self.confirm_apply, version)
                if self._stop.is_set():
                    return

            self._set_status(Status.APPLY_UPDATE, "Applying update...")
            applied = self.api.apply_update(
                CloseOptions(
                    must_close_process_names=self.must_close_process_names,
                    close_timeout_seconds=self.close_timeout_seconds,
                )
            )
            if not applied.is_success:
                self._report_error(f"apply spawn failed: {applied.error_msg}")
            if self._wait(30.0):
                return

    def _try_self_update(self) -> None:
        check = self.api.check_self_update(cancel_event=self._stop)
        if self._stop.is_set() or not check.is_success or not check.data or not check.data.need_update:
            return
        updated = self.api.self_update(cancel_event=self._stop)
        if not updated.is_success:
            self._report_error(updated.error_msg)

    def _call_confirm(
        self,
        callback: Optional[Callable[[str, threading.Event], None]],
        version: str,
    ) -> None:
        if callback is None:
            return
        try:
            callback(version, self._stop)
        except Exception as exc:  # noqa: BLE001 - 回调异常不得打断循环
            self._report_error(f"confirm handler error: {exc}")

    def _set_status(self, status: str, message: str) -> None:
        with self._lock:
            self._status = status
            callback = self.on_status_change
        if callback is not None:
            try:
                callback(status, message)
            except Exception:  # noqa: BLE001
                pass

    def _report_error(self, message: str) -> None:
        with self._lock:
            if self._error == message:
                return
            self._error = message
            callback = self.on_error
        if callback is not None:
            try:
                callback(message)
            except Exception:  # noqa: BLE001
                pass

    def _clear_error(self) -> None:
        with self._lock:
            had_error = self._error != ""
            self._error = ""
            callback = self.on_error
        if had_error and callback is not None:
            try:
                callback("")
            except Exception:  # noqa: BLE001
                pass

    def _wait(self, seconds: float) -> bool:
        return self._stop.wait(seconds)

    def _idle_delay(self) -> float:
        interval = self.idle_poll_interval
        if not interval or interval <= 0:
            interval = 300.0
        if interval < 60.0:
            interval = 60.0
        return interval + random.random() * 60.0