import base64
import hashlib
import json
import importlib.util
import tempfile
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("release_metadata", ROOT / "build/release.py")
RELEASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RELEASE)


class PackagingTests(unittest.TestCase):
    def test_stable_release_tags(self):
        self.assertEqual(RELEASE.release_tag("v1.2.3"), "1.2.3")
        self.assertEqual(RELEASE.release_tag("v0.0.0"), "0.0.0")
        self.assertEqual(RELEASE.release_tag("v65536.0.0"), "65536.0.0")
        for tag in ("1.2.3", "v1.2", "v1.2.3-alpha", "v01.2.3", "v1.2.3\n", "v1.2.3\r\n", "v1.2.3;echo bad", "v١.٢.٣", "v-1.0.0"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                RELEASE.release_tag(tag)

    def test_version_validation(self):
        self.assertEqual(RELEASE.release_version("1.2.3"), "1.2.3")
        for version in ("dev", "v1.2.3", "1.2.3+build", "1.2.3-rc.1", "1.2.3 ", "1.2.03"):
            with self.assertRaises(ValueError):
                RELEASE.release_version(version)

    def test_checksums_are_exact_and_reproducible(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            assets = {"LiuguangTV-1.2.3-windows-amd64.zip": b"portable"}
            for name, data in assets.items():
                (root / name).write_bytes(data)
            (root / "unrelated.exe").write_bytes(b"must not ship")
            output = RELEASE.write_checksums(root, list(reversed(assets)))
            expected = "".join(f"{hashlib.sha256(assets[name]).hexdigest()}  {name}\n" for name in sorted(assets))
            self.assertEqual(output.read_bytes(), expected.encode("ascii"))
            self.assertNotIn("unrelated", output.read_text())
            self.assertEqual(RELEASE.write_checksums(root, list(assets)).read_text(), expected)

    def test_checksum_rejects_missing_or_unsafe_assets(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("../outside.exe", "a\nb.exe", "a b.exe", "SHA256SUMS.txt"):
                with self.assertRaises(ValueError):
                    RELEASE.write_checksums(root, [name])
            with self.assertRaises(ValueError):
                RELEASE.write_checksums(root, [])
            with self.assertRaises(FileNotFoundError):
                RELEASE.write_checksums(root, ["missing.exe"])

    def test_release_notes_match_across_platforms(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            targets = ("windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64")
            for target in targets:
                (root / f"update-{target}.json").write_text(json.dumps({"notes": "### 更新\n- 修复问题"}, ensure_ascii=False), encoding="utf-8")
            output = root / "notes.md"
            RELEASE.write_release_notes(root, output)
            self.assertEqual(output.read_text(encoding="utf-8"), "### 更新\n- 修复问题\n")
            (root / "update-linux-amd64.json").write_text('{"notes": "other"}', encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "differ"):
                RELEASE.write_release_notes(root, output)
            (root / "update-linux-amd64.json").write_text('{"notes": ""}', encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "Missing"):
                RELEASE.write_release_notes(root, output)

    def test_portable_and_mygo_installer_packaging(self):
        package = (ROOT / "build/package_release.py").read_text(encoding="utf-8")
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        self.assertIn('zipped.writestr("portable.flag", "")', package)
        self.assertIn("liuguangtv-portable-{version}-windows-{arch}.zip", package)
        self.assertIn('name: Windows-amd64, runner: windows-2022, platform: "windows/amd64"', workflow)
        self.assertIn('name: Windows-arm64, runner: windows-11-arm, platform: "windows/arm64"', workflow)
        self.assertIn("go tool mygo build", workflow)
        self.assertFalse((ROOT / "build/package_unix.py").exists())
        self.assertFalse((ROOT / "build/windows/package.ps1").exists())

    def test_mygo_cli_matches_module_version(self):
        module = (ROOT / "go.mod").read_text(encoding="utf-8")
        self.assertIn("tool github.com/egoist/mygo/cmd/mygo", module)
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        self.assertEqual(workflow.count("go tool mygo build -platform"), 1)
        self.assertNotIn("github.com/egoist/mygo/cmd/mygo@", workflow)
        self.assertIn("This tag predates portable.flag packaging", workflow)

    def test_update_prerequisites_are_checked_before_builds(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        metadata = workflow.split("\n  icons:", 1)[0]
        self.assertIn("MYGO_UPDATER_PRIVATE_KEY must be configured before releasing", metadata)
        self.assertIn('python release-tools/build/release.py validate-changelog "$RELEASE_VERSION" CHANGELOG.md', metadata)
        self.assertIn("ref: ${{ github.workflow_sha }}", metadata)

    def test_changelog_requires_one_nonempty_version_section(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "CHANGELOG.md"
            path.write_text("# Updates\n\n## 1.2.3\n### Fixed\n- A fix\n\n## 1.2.2\nOlder notes\n", encoding="utf-8")
            RELEASE.validate_changelog("1.2.3", path)
            for content in ("## 1.2.2\nOther version", "## 1.2.3\n\n## 1.2.2\nOlder notes", "## 1.2.3\nNotes\n## 1.2.3\nDuplicate"):
                path.write_text(content, encoding="utf-8")
                with self.assertRaises(ValueError):
                    RELEASE.validate_changelog("1.2.3", path)

    def test_ci_version_config_rejects_invalid_version(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory) / "mygo.json"
            config.write_text('{"version":"old","updates":{"publicKey":"unchanged"}}', encoding="utf-8")
            command = [sys.executable, str(ROOT / "build/release.py"), "set-mygo-version"]
            subprocess.run(command + ["1.2.3", str(config)], check=True)
            self.assertEqual(json.loads(config.read_text(encoding="utf-8")), {"version": "1.2.3", "updates": {"publicKey": "unchanged"}})
            self.assertNotEqual(subprocess.run(command + ["1.2.3-rc", str(config)], capture_output=True).returncode, 0)
            self.assertEqual(json.loads(config.read_text(encoding="utf-8"))["version"], "1.2.3")

    def test_mygo_update_configuration_and_portable_marker(self):
        config = json.loads((ROOT / "mygo.json").read_text(encoding="utf-8"))
        self.assertEqual(config["main"], "./cmd/liuguangtv")
        self.assertEqual(config["updates"]["github"], "tangqiu0205/liuguang-tv")
        self.assertEqual(len(base64.b64decode(config["updates"]["publicKey"], validate=True)), 32)
        self.assertIn("portable.flag", (ROOT / "build/package_release.py").read_text(encoding="utf-8"))

    def test_command_entry_and_build_target(self):
        self.assertTrue((ROOT / "cmd/liuguangtv/main.go").is_file())
        self.assertTrue((ROOT / "cmd/liuguangtv/version.go").is_file())
        self.assertTrue((ROOT / "internal/app/run.go").is_file())
        build = (ROOT / "build.ps1").read_text(encoding="utf-8")
        self.assertIn("./cmd/liuguangtv", build)
        self.assertIn("-X main.version=$Version", build)

    def test_workflow_publishes_only_after_successful_build(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        for required in ('tags: ["v*"]', 'needs: [metadata, build]', 'contents: write', 'go run ./build/verifyupdate', 'sha256sum --check SHA256SUMS.txt', 'go-version-file: go.mod', 'python-version: "3.12"'):
            self.assertIn(required, workflow)
        self.assertIn('build/package_release.py', workflow)
        self.assertIn('needs: [metadata, icons]', workflow)
        self.assertNotIn('test-windows:', workflow)
        self.assertIn('Windows-amd64 Windows-arm64', workflow)
        self.assertIn('ref: ${{ needs.metadata.outputs.commit }}', workflow)
        self.assertNotIn('needs.build.outputs', workflow)
        self.assertIn('secrets.MYGO_UPDATER_PRIVATE_KEY', workflow)
        self.assertIn('ref: ${{ github.workflow_sha }}', workflow)
        self.assertIn('python release-tools/build/publish_release.py', workflow)
        publisher = (ROOT / 'build/publish_release.py').read_text(encoding='utf-8')
        self.assertIn('--verify-tag', publisher)
        self.assertIn('--notes-file', publisher)
        self.assertIn('release-notes publish-assets release-notes.md', workflow)
        self.assertNotIn('ci/update-test', workflow)
        self.assertLess(workflow.index('name: Verify signed updates'), workflow.index('name: Publish release'))
        self.assertEqual(workflow.count('--clobber'), 1)
        self.assertIn('publish_release.py draft "$RELEASE_TAG" CHANGELOG.md', workflow)
        self.assertIn('publish_release.py publish "$RELEASE_TAG" publish-assets release-notes.md', workflow)
        self.assertLess(workflow.index('name: Create or reuse release'), workflow.index('\n  build:'))
        self.assertEqual(workflow.count('name: Upload packages to release'), 1)
        # Platform checksum files share a filename: only the combined one is published.
        self.assertIn('! -name SHA256SUMS.txt -print0', workflow)
        self.assertNotIn('secrets.GH_TOKEN', workflow)  # GITHUB_TOKEN suffices.


if __name__ == "__main__":
    unittest.main()
