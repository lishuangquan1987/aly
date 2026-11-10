from __future__ import annotations

import os
import tempfile
import time
import unittest

from alyclient import (
    API,
    CleanOptions,
    CloseOptions,
    default_path,
)

from helpers import fake_client, scenario


class APITests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.fake = fake_client()

    def test_check_update_available(self) -> None:
        with scenario("update_available"):
            response = API(self.fake).check_update()
        self.assertTrue(response.is_success, response.error_msg)
        self.assertTrue(response.data.has_update)
        self.assertTrue(response.data.need_download_update)
        self.assertEqual(response.data.new_version, "1.0.1")
        self.assertFalse(response.data.force_update)
        self.assertEqual(len(response.data.release_notes or []), 2)

    def test_check_update_already_latest_and_offline(self) -> None:
        for name in ("already_latest", "offline"):
            with self.subTest(name=name), scenario(name):
                response = API(self.fake).check_update()
            self.assertTrue(response.is_success, response.error_msg)
            self.assertFalse(response.data.has_update)

    def test_download_progress(self) -> None:
        progress = []
        with scenario("update_available"):
            response = API(self.fake).download_update(on_progress=progress.append)
        self.assertTrue(response.is_success, response.error_msg)
        self.assertIsNone(response.data)
        self.assertEqual(len(progress), 1)
        self.assertEqual(progress[0].file, "app.exe")
        self.assertEqual(progress[0].ratio, 1.0)
        self.assertEqual(progress[0].status, "DONE")
        self.assertEqual(progress[0].downloaded_bytes, 1000)

    def test_download_partial_failure(self) -> None:
        with scenario("download_partial_failure"):
            response = API(self.fake).download_update()
        self.assertFalse(response.is_success)
        self.assertIn("checksum mismatch", response.error_msg)

    def test_download_stall_watchdog(self) -> None:
        started = time.monotonic()
        with scenario("download_stall"):
            response = API(self.fake).download_update(stall_timeout=1)
        self.assertFalse(response.is_success)
        self.assertIn("stalled", response.error_msg)
        self.assertLess(time.monotonic() - started, 4.0)

    def test_apply_returns_immediately(self) -> None:
        started = time.monotonic()
        with scenario("update_available"):
            response = API(self.fake).apply_update(
                CloseOptions(must_close_process_names="A,B", close_timeout_seconds=45)
            )
        self.assertTrue(response.is_success, response.error_msg)
        self.assertLess(time.monotonic() - started, 2.0)

    def test_self_update_flow(self) -> None:
        with scenario("self_update"):
            api = API(self.fake)
            check = api.check_self_update()
            updated = api.self_update()
        self.assertTrue(check.is_success and check.data.need_update)
        self.assertTrue(updated.is_success and updated.data.updated)
        self.assertEqual(updated.data.old_path, "aly-client.exe.old")

    def test_rollback_flow(self) -> None:
        with scenario("rollback"):
            api = API(self.fake)
            versions = api.list_rollback_versions()
            rolled = api.rollback("1.0.0")
        self.assertTrue(versions.is_success)
        self.assertEqual(versions.data.versions, ["1.0.0", "0.9.0"])
        self.assertTrue(rolled.is_success)
        self.assertEqual(rolled.data.version, "1.0.0")

    def test_check_diff_and_clean(self) -> None:
        with scenario("check_diff"):
            diff = API(self.fake).check_diff()
        self.assertTrue(diff.is_success)
        self.assertEqual(diff.data.files[0].path, "app.exe")

        with scenario("clean_all"):
            cleaned = API(self.fake).clean(CleanOptions(all=True))
        self.assertTrue(cleaned.is_success)
        self.assertEqual(cleaned.data.mode, "all")

    def test_missing_executable(self) -> None:
        missing = os.path.join(tempfile.mkdtemp(prefix="aly-py-missing-"), "aly-client.exe")
        response = API(missing).check_update(timeout=3)
        self.assertFalse(response.is_success)
        self.assertTrue(response.error_msg)

    def test_default_path(self) -> None:
        self.assertIn("UpdateFolder", default_path())


if __name__ == "__main__":
    unittest.main()