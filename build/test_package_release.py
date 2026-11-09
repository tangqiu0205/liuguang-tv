import hashlib
import json
import tempfile
import unittest
import zipfile
from pathlib import Path

from package_release import collect

ROOT = Path(__file__).resolve().parents[1]


class PackageReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "mygo"
        self.output = self.root / "release"

    def stage(self, target, delta=False):
        system, arch = target.split("-")
        stage = self.source / target
        stage.mkdir(parents=True)
        archive = f"liuguangtv-1.2.3-{target}.tar.gz"
        (stage / archive).write_bytes(b"original signed MyGo archive")
        manifest = {"version": "1.2.3", "url": "https://example.org/" + archive, "size": 32, "signature": "signature"}
        if delta:
            name = f"liuguangtv-1.2.2-to-1.2.3-{target}.delta"
            (stage / name).write_bytes(b"original signed delta")
            manifest["deltas"] = [{"from": "1.2.2", "url": "https://example.org/" + name}]
        if system == "windows":
            (stage / "LiuguangTV.exe").write_bytes(b"executable")
            (stage / ".mygo-files").write_text("LiuguangTV.exe\nresources\n", encoding="utf-8")
            (stage / "resources").mkdir()
            (stage / "resources/icon.png").write_bytes(b"icon")
            (stage / "install.ps1").write_text("# MyGo install helper", encoding="utf-8")
            installer = f"liuguangtv-setup-1.2.3-{arch}.exe"
            (stage / "data").mkdir()
            (stage / "data/state.json").write_text("private", encoding="utf-8")
            (stage / "state.json").write_text("also private", encoding="utf-8")
        elif system == "darwin":
            installer = f"liuguangtv-1.2.3-{arch}.dmg"
        else:
            installer = f"liuguangtv_1.2.3_{arch}.deb"
        (stage / installer).write_bytes(b"native installer")
        (stage / f"update-{target}.json").write_text(json.dumps(manifest), encoding="utf-8")
        return stage

    def test_windows_portable_root_layout_preserves_native_installed_files(self):
        for arch in ("amd64", "arm64"):
            target = f"windows-{arch}"
            stage = self.stage(target, delta=True)
            output = self.output / arch
            assets = collect("1.2.3", target.replace("-", "/"), self.source, output)
            self.assertEqual(len(assets), 5)
            archive = output / f"liuguangtv-portable-1.2.3-{target}.zip"
            with zipfile.ZipFile(archive) as zipped:
                self.assertEqual(set(zipped.namelist()), {"LiuguangTV.exe", ".mygo-files", "resources/icon.png", "install.ps1", "portable.flag"})
                self.assertEqual(zipped.read("LiuguangTV.exe"), b"executable")
            self.assertFalse((stage / "portable.flag").exists())
            self.assert_checksums(output, assets)

    def test_native_unix_packages_are_copied_without_repacking(self):
        for system in ("darwin", "linux"):
            for arch in ("amd64", "arm64"):
                target = f"{system}-{arch}"
                self.stage(target)
                output = self.output / target
                assets = collect("1.2.3", f"{system}/{arch}", self.source, output)
                self.assertEqual(len(assets), 3)
                for asset in assets:
                    self.assertEqual(asset.read_bytes(), (self.source / target / asset.name).read_bytes())
                self.assert_checksums(output, assets)

    def test_collects_both_windows_architectures_and_optional_deltas(self):
        self.stage("windows-amd64", delta=True)
        self.stage("windows-arm64", delta=False)
        assets = collect("1.2.3", "windows/amd64,windows/arm64", self.source, self.output)
        self.assertEqual(len(assets), 9)
        self.assertEqual(len([p for p in assets if p.suffix == ".delta"]), 1)
        self.assert_checksums(self.output, assets)

    def test_missing_referenced_delta_fails(self):
        stage = self.stage("linux-amd64", delta=True)
        next(stage.glob("*.delta")).unlink()
        with self.assertRaises(FileNotFoundError):
            collect("1.2.3", "linux/amd64", self.source, self.output)

    def test_rejects_invalid_platform_version_or_installed_payload(self):
        for version, platforms in (("bad", "linux/amd64"), ("1.2.3", "linux/386"), ("1.2.3", "linux/amd64,linux/amd64")):
            with self.assertRaises(ValueError):
                collect(version, platforms, self.source, self.output)
        stage = self.stage("windows-amd64")
        for name in ("../outside", "data", "DATA", "portable.flag", "c:outside", "liuguangtv.EXE"):
            (stage / ".mygo-files").write_text("LiuguangTV.exe\n" + name, encoding="utf-8")
            with self.assertRaises(ValueError):
                collect("1.2.3", "windows/amd64", self.source, self.root / name.replace("/", "-").replace(".", "_"))

    def test_workflow_matches_reference_build_matrix_and_gates_publication(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        for required in ('platform: "windows/amd64,windows/arm64"', "runner: ubuntu-24.04", "macos-15-intel", "macos-15", "ubuntu-24.04-arm", "test-windows:",
                         "needs: [metadata, build, test-windows]", "pattern: packages-*", "go run ./build/verifyupdate", "--clobber", "go test ./...", "go vet ./..."):
            self.assertIn(required, workflow)
        for obsolete in ("package_unix.py", "build/windows/package.ps1", "windows-11-arm", "-eq 24", "-eq 4", "continue-on-error"):
            self.assertNotIn(obsolete, workflow)
        config = json.loads((ROOT / "mygo.json").read_text(encoding="utf-8"))
        self.assertNotIn("deltas", config["updates"])
        self.assertEqual(config["updates"]["publicKey"], "XdHqUMVNQc9M5qhZZQKITI4mr/t2/4jdiXmxsDoo8OA=")
        self.assertNotIn("buildCommand", config)  # No mpv preparation or bundled decoder.

    def assert_checksums(self, output, assets):
        expected = "".join(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n" for path in sorted(assets))
        self.assertEqual((output / "SHA256SUMS.txt").read_text(), expected)


if __name__ == "__main__":
    unittest.main()
