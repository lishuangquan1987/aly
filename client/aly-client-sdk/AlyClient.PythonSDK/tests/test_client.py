from __future__ import annotations

import threading
import time
import unittest

from alyclient import Status, UpdateClient

from helpers import fake_client, scenario, wait_for


class LifecycleTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.fake = fake_client()

    def test_force_update_runs_full_chain(self) -> None:
        with scenario("force_update"):
            statuses = []
            lock = threading.Lock()

            def on_status(status: str, _message: str) -> None:
                with lock:
                    statuses.append(status)

            client = UpdateClient(self.fake)
            client.on_status_change = on_status
            client.start()
            wait_for(lambda: client.status == Status.APPLY_UPDATE, 5)
            self.assertTrue(client.stop(5))

        with lock:
            got = list(statuses)
        self.assertTrue(got)
        self.assertEqual(got[0], Status.DISCOVERED_UPDATE)
        self.assertIn(Status.DOWNLOADING_UPDATE, got)
        self.assertIn(Status.DOWNLOADED_UPDATE, got)
        self.assertEqual(got[-1], Status.APPLY_UPDATE)
        self.assertEqual(client.last_error, "")
        self.assertFalse(client.is_running)

    def test_non_force_waits_for_confirmations(self) -> None:
        with scenario("update_available"):
            downloads = []
            applies = []
            client = UpdateClient(self.fake)

            def confirm_download(version: str, _stop: threading.Event) -> None:
                downloads.append(version)

            def confirm_apply(version: str, _stop: threading.Event) -> None:
                applies.append(version)

            client.confirm_download = confirm_download
            client.confirm_apply = confirm_apply
            client.start()
            wait_for(lambda: client.status == Status.APPLY_UPDATE, 5)
            self.assertTrue(client.stop(5))

        self.assertEqual(downloads, ["1.0.1"])
        self.assertEqual(applies, ["1.0.1"])

    def test_offline_stays_idle(self) -> None:
        with scenario("offline"):
            client = UpdateClient(self.fake)
            asked = []
            client.confirm_download = lambda version, _stop: asked.append(version)
            client.start()
            time.sleep(0.5)
            self.assertEqual(client.status, Status.NONE)
            self.assertTrue(client.stop(5))
        self.assertEqual(asked, [])

    def test_stop_interrupts_pending_confirmation(self) -> None:
        with scenario("update_available"):
            client = UpdateClient(self.fake)

            def confirm_download(_version: str, stop: threading.Event) -> None:
                stop.wait(30)

            client.confirm_download = confirm_download
            client.start()
            wait_for(lambda: client.status == Status.DISCOVERED_UPDATE, 5)

            started = time.monotonic()
            stopped = client.stop(5)
            elapsed = time.monotonic() - started
        self.assertTrue(stopped)
        self.assertLess(elapsed, 2.0)

    def test_stop_is_idempotent(self) -> None:
        with scenario("offline"):
            client = UpdateClient(self.fake)
            client.start()
            time.sleep(0.1)
            self.assertTrue(client.stop(5))
            self.assertTrue(client.stop(5))
            self.assertFalse(client.is_running)


if __name__ == "__main__":
    unittest.main()