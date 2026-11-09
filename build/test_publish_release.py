import importlib.util
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("publish_release", ROOT / "build/publish_release.py")
PUBLISH = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PUBLISH)


class FakeGitHub:
    def __init__(self, release=None):
        self.release = release
        self.created = False
        self.uploads = []
        self.published = False
        self.upload_error = False

    def find(self, tag):
        return self.release

    def create(self, tag, notes_file):
        self.created = True
        self.release = {"draft": True, "body": notes_file.read_text(encoding="utf-8")}

    def upload(self, tag, file):
        if self.upload_error:
            raise RuntimeError("upload failed")
        self.uploads.append(file.name)

    def publish(self, tag, notes_file):
        self.published = True
        self.release["draft"] = False
        self.release["body"] = notes_file.read_text(encoding="utf-8")


class PublishReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name) / "assets"
        self.root.mkdir()
        self.notes = Path(self.temporary.name) / "notes.md"
        self.notes.write_text("版本说明\n", encoding="utf-8")
        self.changelog = Path(self.temporary.name) / "CHANGELOG.md"
        self.changelog.write_text("# Updates\n\n## 1.2.3\n版本说明\n\n## 1.2.2\nPrevious\n", encoding="utf-8")
        self.checksum = self.root / "SHA256SUMS.txt"
        self.checksum.write_text("verified checksums\n", encoding="utf-8")

    def test_creates_draft_before_builds_with_changelog_notes(self):
        gh = FakeGitHub()
        PUBLISH.prepare_release(gh, "v1.2.3", self.changelog)
        self.assertTrue(gh.created)
        self.assertEqual(gh.release, {"draft": True, "body": "版本说明\n"})
        self.assertEqual(gh.uploads, [])
        self.assertFalse(gh.published)

    def test_reuses_drafts_and_published_releases_without_hash_or_notes_guard(self):
        for draft in (True, False):
            with self.subTest(draft=draft):
                gh = FakeGitHub({"draft": draft, "body": "Old notes", "assets": [{"name": "app.zip", "digest": "old"}]})
                PUBLISH.prepare_release(gh, "v1.2.3", self.changelog)
                self.assertFalse(gh.created)
                self.assertEqual(gh.uploads, [])

    def test_missing_changelog_section_fails_before_creating_release(self):
        gh = FakeGitHub()
        with self.assertRaises(ValueError):
            PUBLISH.prepare_release(gh, "v9.9.9", self.changelog)
        self.assertFalse(gh.created)

    def test_only_uploads_combined_checksums_after_platform_jobs(self):
        gh = FakeGitHub({"draft": True, "body": "Old notes"})
        (self.root / "app.zip").write_bytes(b"already uploaded by the platform job")
        PUBLISH.publish_release(gh, "v1.2.3", self.root, self.notes)
        self.assertEqual(gh.uploads, ["SHA256SUMS.txt"])
        self.assertTrue(gh.published)
        self.assertEqual(gh.release["body"], "版本说明\n")

    def test_allows_republishing_existing_release(self):
        gh = FakeGitHub({"draft": False, "body": "Old notes"})
        PUBLISH.publish_release(gh, "v1.2.3", self.root, self.notes)
        self.assertTrue(gh.published)

    def test_failed_checksum_upload_does_not_publish(self):
        gh = FakeGitHub({"draft": True})
        gh.upload_error = True
        with self.assertRaisesRegex(RuntimeError, "upload failed"):
            PUBLISH.publish_release(gh, "v1.2.3", self.root, self.notes)
        self.assertFalse(gh.published)

    def test_requires_combined_checksums_and_nonempty_notes(self):
        gh = FakeGitHub({"draft": True})
        self.notes.write_text("", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "notes"):
            PUBLISH.publish_release(gh, "v1.2.3", self.root, self.notes)
        self.notes.write_text("Notes", encoding="utf-8")
        self.checksum.unlink()
        with self.assertRaisesRegex(ValueError, "checksums"):
            PUBLISH.publish_release(gh, "v1.2.3", self.root, self.notes)
        self.assertFalse(gh.published)
        self.assertEqual(gh.uploads, [])

    def test_github_uses_clobber_and_publishes_latest_with_notes(self):
        gh = PUBLISH.GitHub()
        with mock.patch.object(gh, "run") as run:
            gh.upload("v1.2.3", self.checksum)
            self.assertIn("--clobber", run.call_args.args)
            gh.publish("v1.2.3", self.notes)
            self.assertIn("--draft=false", run.call_args.args)
            self.assertIn("--latest", run.call_args.args)
            self.assertIn("--notes-file", run.call_args.args)

    def test_github_command_failure_is_not_accepted_as_success(self):
        with mock.patch.object(PUBLISH.subprocess, "run", return_value=SimpleNamespace(returncode=1, stderr="network error")):
            with self.assertRaisesRegex(RuntimeError, "network error"):
                PUBLISH.GitHub().upload("v1.2.3", self.checksum)
            with self.assertRaisesRegex(RuntimeError, "network error"):
                PUBLISH.GitHub().find("v1.2.3")


if __name__ == "__main__":
    unittest.main()
