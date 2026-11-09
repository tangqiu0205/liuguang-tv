#!/usr/bin/env python3
"""Collect native MyGo packages and make the Windows portable ZIP (no mpv)."""
import argparse
import json
import re
import shutil
import zipfile
from pathlib import Path
from urllib.parse import urlparse

from release import release_version, write_checksums

TARGETS = {f"{system}-{arch}" for system in ("windows", "darwin", "linux") for arch in ("amd64", "arm64")}


def portable_windows(source: Path, output: Path, version: str, arch: str) -> Path:
    # Use MyGo's installed file list, not everything in the build directory:
    # installers, update archives and local data must never enter the ZIP.
    names = source.joinpath(".mygo-files").read_text(encoding="utf-8").splitlines()
    if "LiuguangTV.exe" not in names:
        raise ValueError("MyGo installed file list is missing LiuguangTV.exe")
    paths = [source / ".mygo-files"]
    seen = set()
    for name in names:
        folded = name.casefold()
        if not name or folded in seen or folded in {".", "..", ".mygo-files", "data", "portable.flag", "liuguangtv-portable.mode"} or any(c in name for c in "/\\:\0"):
            raise ValueError(f"Unsafe installed entry: {name!r}")
        seen.add(folded)
        entry = source / name
        if entry.is_symlink() or not entry.exists():
            raise ValueError(f"Missing or linked installed entry: {name}")
        paths.extend(sorted(entry.rglob("*")) if entry.is_dir() else [entry])
    # Match MyGo's Windows distribution, including its optional install helper.
    if (source / "install.ps1").is_file():
        paths.append(source / "install.ps1")
    archive = output / f"liuguangtv-portable-{version}-windows-{arch}.zip"
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as zipped:
        for path in paths:
            if path.is_symlink():
                raise ValueError(f"Linked portable entry: {path}")
            if path.is_file():
                zipped.write(path, path.relative_to(source).as_posix())
        zipped.writestr("portable.flag", "")
    return archive


def collect(version: str, platforms: str, source: Path, output: Path) -> list[Path]:
    release_version(version)
    targets = platforms.replace("/", "-").split(",")
    if not targets or len(targets) != len(set(targets)) or any(target not in TARGETS for target in targets):
        raise ValueError("Unsupported or duplicate platform")
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise ValueError("Release output directory must be empty")
    assets = []
    for target in targets:
        system, arch = target.split("-")
        stage = source / target
        manifest_name = f"update-{target}.json"
        manifest = json.loads((stage / manifest_name).read_text(encoding="utf-8"))
        if manifest.get("version") != version:
            raise ValueError(f"Wrong update version for {target}")
        names = [f"liuguangtv-{version}-{target}.tar.gz", manifest_name]
        if system == "windows":
            names.append(f"liuguangtv-setup-{version}-{arch}.exe")
            assets.append(portable_windows(stage, output, version, arch))
        elif system == "darwin":
            names.append(f"liuguangtv-{version}-{arch}.dmg")
        else:
            names.append(f"liuguangtv_{version}_{arch}.deb")
        seen = set()
        for delta in manifest.get("deltas", []):
            base = release_version(delta["from"])
            if base == version or base in seen:
                raise ValueError(f"Duplicate or invalid delta base: {base}")
            seen.add(base)
            name = f"liuguangtv-{base}-to-{version}-{target}.delta"
            if Path(urlparse(delta["url"]).path).name != name:
                raise ValueError(f"Wrong delta filename for {target}")
            names.append(name)
        for name in names:
            if not re.fullmatch(r"[A-Za-z0-9._-]+", name):
                raise ValueError("Invalid release filename")
            path = stage / name
            if not path.is_file() or path.is_symlink():
                raise FileNotFoundError(f"Missing release asset: {path}")
            destination = output / name
            shutil.copyfile(path, destination)
            assets.append(destination)
    write_checksums(output, [asset.name for asset in assets])
    return assets


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--platform", required=True)
    parser.add_argument("--source", type=Path, default=Path("dist/mygo"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        collect(args.version, args.platform, args.source, args.output)
    except (OSError, ValueError, KeyError) as error:
        parser.exit(2, f"{error}\n")


if __name__ == "__main__":
    main()
