#!/usr/bin/env python3
"""Validate stable release metadata and hash explicitly selected assets (stdlib only)."""
import argparse
import hashlib
import json
import re
from pathlib import Path


VERSION_PATTERN = re.compile(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", re.ASCII)


def release_version(version: str) -> str:
    match = VERSION_PATTERN.fullmatch(version)
    if not match:
        raise ValueError("Expected stable MAJOR.MINOR.PATCH without leading zeroes")
    return version


def release_tag(tag: str) -> str:
    if not tag.startswith("v"):
        raise ValueError("Expected a stable tag such as v1.2.3")
    return release_version(tag[1:])


def validate_changelog(version: str, path: Path) -> str:
    heading = f"## {release_version(version)}"
    lines = path.read_text(encoding="utf-8").splitlines()
    positions = [i for i, line in enumerate(lines) if line == heading]
    if len(positions) != 1:
        raise ValueError(f"Expected exactly one changelog section: {heading}")
    start = positions[0] + 1
    end = next((i for i in range(start, len(lines)) if lines[i].startswith("## ")), len(lines))
    notes = "\n".join(lines[start:end]).strip()
    if not notes:
        raise ValueError(f"Empty changelog section: {heading}")
    return notes + "\n"


def write_checksums(directory: Path, filenames: list[str]) -> Path:
    lines = []
    for name in sorted(set(filenames)):
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", name, re.ASCII) or name == "SHA256SUMS.txt":
            raise ValueError(f"Invalid release asset filename: {name!r}")
        path = directory / name
        with path.open("rb") as source:
            digest = hashlib.file_digest(source, "sha256").hexdigest()
        lines.append(f"{digest}  {name}\n")
    if not lines:
        raise ValueError("At least one release asset is required")
    output = directory / "SHA256SUMS.txt"
    output.write_text("".join(lines), encoding="utf-8", newline="\n")
    return output


def write_release_notes(directory: Path, output: Path) -> Path:
    targets = ("windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64")
    notes = []
    for target in targets:
        manifest = json.loads((directory / f"update-{target}.json").read_text(encoding="utf-8"))
        note = manifest.get("notes")
        if not isinstance(note, str) or not note.strip():
            raise ValueError(f"Missing release notes for {target}")
        notes.append(note.strip())
    if len(set(notes)) != 1:
        raise ValueError("Release notes differ between update manifests")
    output.write_text(notes[0] + "\n", encoding="utf-8", newline="\n")
    return output


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("validate-tag").add_argument("tag")
    commands.add_parser("validate-version").add_argument("version")
    changelog = commands.add_parser("validate-changelog")
    changelog.add_argument("version")
    changelog.add_argument("path", type=Path)
    config = commands.add_parser("set-mygo-version")
    config.add_argument("version")
    config.add_argument("config", type=Path)
    notes = commands.add_parser("release-notes")
    notes.add_argument("directory", type=Path)
    notes.add_argument("output", type=Path)
    checksum = commands.add_parser("checksums")
    checksum.add_argument("directory", type=Path)
    checksum.add_argument("filenames", nargs="+")
    args = parser.parse_args()
    try:
        if args.command == "validate-tag":
            print(release_tag(args.tag))
        elif args.command == "validate-version":
            print(release_version(args.version))
        elif args.command == "validate-changelog":
            validate_changelog(args.version, args.path)
        elif args.command == "set-mygo-version":
            version = release_version(args.version)
            config = json.loads(args.config.read_text(encoding="utf-8"))
            config["version"] = version
            args.config.write_text(json.dumps(config, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        elif args.command == "release-notes":
            print(write_release_notes(args.directory, args.output))
        else:
            print(write_checksums(args.directory, args.filenames))
    except (ValueError, OSError) as error:
        parser.exit(2, f"{error}\n")


if __name__ == "__main__":
    main()
