import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from contextlib import ExitStack
import io
import urllib.error

import github
import entrypoint
import server
from common import changelog, message, validate


def manifest(version="v4"):
    return {"version": version, "commit": "a" * 40, "image": "ghcr.io/sitleman/challenger@sha256:" + "b" * 64,
            "entries": {"a.json": {"type": "feature", "text": "Новая команда"},
                        "b.json": {"type": "fix", "text": "Исправлены итоги"}},
            "base": "v1", "notes": "Новая команда"}


class ReleaseTests(unittest.TestCase):
    def test_aggregate_since_production_not_last_stage(self):
        previous = {"entries": {"old.json": {"type": "fix", "text": "Старое"}}}
        target = manifest()
        target["entries"].update(previous["entries"])
        result = changelog(previous, target)
        self.assertIn("Новая команда", result)
        self.assertIn("Исправлены итоги", result)
        self.assertNotIn("Старое", result)

    def test_internal_and_duplicate_changes(self):
        target = manifest()
        target["entries"]["c.json"] = {"type": "feature", "text": "Новая команда"}
        target["entries"]["d.json"] = {"type": "internal", "text": "Docker"}
        result = changelog({}, target)
        self.assertEqual(result.count("Новая команда"), 1)
        self.assertNotIn("Docker", result)
        self.assertIn("Технические", changelog(target, target))

    def test_message_limit_uses_utf16(self):
        with self.assertRaises(ValueError):
            message("v1", "😀" * 2048)
        self.assertIn("v1", message("v1", "Обновление"))

    def test_mutable_image_and_invalid_versions_rejected(self):
        for key, value in [("version", "v0"), ("version", "v2;id"), ("image", "ghcr.io/sitleman/challenger:latest")]:
            with self.subTest(key=key, value=value):
                target = manifest()
                target[key] = value
                with self.assertRaises(ValueError):
                    validate(target)

    def test_deploy_rejects_stale_candidate_before_docker(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest("v2")))
            with patch.object(server, "compose") as compose:
                with self.assertRaisesRegex(ValueError, "production changed"):
                    server.deploy(root, {}, manifest(), "production")
                compose.assert_not_called()

    def test_redeploy_does_not_backup_or_restart(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest()))
            with patch.object(server, "healthy"), patch.object(server, "compose") as compose:
                server.deploy(root, {}, manifest(), "production")
                compose.assert_not_called()

    def test_failed_release_blocks_next_deploy(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest("v1")))
            (root / "pending.json").write_text("{}")
            with self.assertRaisesRegex(ValueError, "unfinished"):
                server.deploy(root, {}, manifest(), "production")

    def test_ambiguous_notification_not_retried(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest()))
            receipts = root / "notifications"
            receipts.mkdir()
            (receipts / "v4-123.json").write_text('{"status":"sending"}')
            with patch.object(server, "healthy"), patch.object(server, "validate_config"), patch("urllib.request.urlopen") as send:
                with self.assertRaisesRegex(ValueError, "ambiguous"):
                    server.notify(root, {"chat_id": 123}, manifest())
                send.assert_not_called()

    def test_sent_notification_not_repeated(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest()))
            receipts = root / "notifications"
            receipts.mkdir()
            (receipts / "v4-123.json").write_text('{"status":"sent","message_id":42}')
            with patch.object(server, "healthy"), patch.object(server, "validate_config"), patch("urllib.request.urlopen") as send:
                server.notify(root, {"chat_id": 123}, manifest())
                send.assert_not_called()

    def test_deploy_backs_up_and_commits_only_after_health(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory) / "state"
            root.mkdir()
            volume = Path(directory) / "volume"
            volume.mkdir()
            (volume / "challenger.db").write_text("old database")
            (root / "current.json").write_text(json.dumps(manifest("v1")))
            compose = stack.enter_context(patch.object(server, "compose", return_value="container"))
            stack.enter_context(patch.object(server, "volume_path", return_value=volume))
            stack.enter_context(patch.object(server, "validate_config"))
            def ready(*args):
                self.assertEqual(json.loads((root / "current.json").read_text())["version"], "v1")
                self.assertTrue((root / "pending.json").exists())
            stack.enter_context(patch.object(server, "healthy", side_effect=ready))
            server.deploy(root, {}, manifest(), "production")
            self.assertEqual(json.loads((root / "current.json").read_text())["version"], "v4")
            self.assertFalse((root / "pending.json").exists())
            backups = list((root / "backups").glob("*/data.tgz"))
            self.assertEqual(len(backups), 1)
            with server.tarfile.open(backups[0]) as archive:
                self.assertEqual(archive.extractfile("./challenger.db").read(), b"old database")
            operations = [call.args[2] for call in compose.call_args_list]
            self.assertLess(operations.index("pull"), operations.index("stop"))
            self.assertLess(operations.index("stop"), operations.index("up"))

    def test_unhealthy_new_version_is_stopped_and_blocks_following_deploy(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            volume = root / "volume"
            volume.mkdir()
            (root / "current.json").write_text(json.dumps(manifest("v1")))
            compose = stack.enter_context(patch.object(server, "compose", return_value="container"))
            stack.enter_context(patch.object(server, "volume_path", return_value=volume))
            stack.enter_context(patch.object(server, "validate_config"))
            stack.enter_context(patch.object(server, "healthy", side_effect=RuntimeError("not ready")))
            with self.assertRaisesRegex(RuntimeError, "new bot stopped"):
                server.deploy(root, {}, manifest(), "production")
            self.assertEqual(json.loads((root / "current.json").read_text())["version"], "v1")
            self.assertTrue((root / "pending.json").exists())
            self.assertEqual(compose.call_args.args[2:], ("stop", "bot"))

    def test_backup_failure_restarts_previous_version(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest("v1")))
            compose = stack.enter_context(patch.object(server, "compose", return_value="container"))
            stack.enter_context(patch.object(server, "validate_config"))
            stack.enter_context(patch.object(server, "volume_path", side_effect=RuntimeError("disk error")))
            stack.enter_context(patch.object(server, "healthy"))
            with self.assertRaisesRegex(RuntimeError, "disk error"):
                server.deploy(root, {}, manifest(), "production")
            self.assertFalse((root / "pending.json").exists())
            self.assertEqual(compose.call_args.args[1]["version"], "v1")
            self.assertEqual(compose.call_args.args[2], "up")

    def test_send_timeout_preserves_ambiguous_receipt(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest()))
            stack.enter_context(patch.object(server, "healthy"))
            stack.enter_context(patch.object(server, "validate_config"))
            stack.enter_context(patch.object(server, "read_env", return_value={"BOT_TOKEN": "secret"}))
            stack.enter_context(patch("urllib.request.urlopen", side_effect=TimeoutError()))
            with self.assertRaisesRegex(RuntimeError, "ambiguous"):
                server.notify(root, {"chat_id": 123, "env_file": "unused"}, manifest())
            self.assertEqual(json.loads((root / "notifications/v4-123.json").read_text())["status"], "sending")

    def test_explicit_telegram_rejection_can_be_retried(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            (root / "current.json").write_text(json.dumps(manifest()))
            stack.enter_context(patch.object(server, "healthy"))
            stack.enter_context(patch.object(server, "validate_config"))
            stack.enter_context(patch.object(server, "read_env", return_value={"BOT_TOKEN": "secret"}))
            error = urllib.error.HTTPError("hidden", 403, "forbidden", {}, io.BytesIO(b'{"ok":false,"error_code":403}'))
            stack.enter_context(patch("urllib.request.urlopen", side_effect=error))
            with self.assertRaisesRegex(RuntimeError, "Telegram rejected"):
                server.notify(root, {"chat_id": 123, "env_file": "unused"}, manifest())
            self.assertEqual(json.loads((root / "notifications/v4-123.json").read_text())["status"], "rejected")

    def test_missing_approval_gate_fails_closed(self):
        with patch.object(github, "api", return_value={"protection_rules": []}):
            with self.assertRaisesRegex(ValueError, "required reviewer"):
                github.require_approval_gate()

    def test_existing_change_fragment_cannot_be_modified(self):
        with patch.object(github, "git", return_value="M\tchanges/existing.json"):
            with self.assertRaisesRegex(ValueError, "immutable"):
                github.validate_changes("base")

    def test_stage_key_cannot_deploy_production_or_execute_shell(self):
        for command in ("deploy production", "sh", "deploy stage; id", "scp -t /tmp/x"):
            with self.subTest(command=command), patch("sys.argv", ["entrypoint.py", "stage"]), patch.dict("os.environ", {"SSH_ORIGINAL_COMMAND": command}):
                with self.assertRaisesRegex(ValueError, "restricted"):
                    entrypoint.main()

    def test_failed_manual_rollback_blocks_new_deployments(self):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            backup = root / "backups" / "20261105T000000Z-v4"
            backup.mkdir(parents=True)
            (backup / "previous.json").write_text(json.dumps(manifest("v1")))
            (backup / "target.json").write_text(json.dumps(manifest()))
            with server.tarfile.open(backup / "data.tgz", "w:gz"):
                pass
            stack.enter_context(patch.object(server, "run"))
            stack.enter_context(patch.object(server, "compose", side_effect=RuntimeError("docker unavailable")))
            with self.assertRaisesRegex(RuntimeError, "docker unavailable"):
                server.rollback(root, {}, backup.name)
            self.assertEqual(json.loads((root / "pending.json").read_text())["phase"], "rollback")

    def test_prod_baseline_tracks_rollback_instead_of_largest_version(self):
        responses = [[{"ref": "refs/tags/deployed-production", "object": {"sha": "tag"}}],
                     {"message": json.dumps(manifest("v2"))}]
        with patch.object(github, "api", side_effect=responses):
            self.assertEqual(github.production()["version"], "v2")



if __name__ == "__main__":
    unittest.main()
