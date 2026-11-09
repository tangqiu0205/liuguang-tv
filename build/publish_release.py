#!/usr/bin/env python3
"""Prepare a release before builds, then publish after verified uploads."""
import argparse
import json
import subprocess
import sys
import tempfile
from pathlib import Path

from release import release_tag, validate_changelog


class GitHub:
    def run(self, *args):
        result = subprocess.run(["gh", *args], capture_output=True, text=True, encoding="utf-8", errors="replace")
        if result.returncode:
            raise RuntimeError(f"gh {args[0]} failed: {result.stderr.strip()}")
        return result

    def find(self, tag):
        # Distinguish an absent release from authentication/network errors;
        # drafts must be visible too, so do not use the public tags endpoint.
        result = self.run("api", "--paginate", "--slurp", "repos/{owner}/{repo}/releases?per_page=100")
        pages = json.loads(result.stdout)
        matches = [release for page in pages for release in page if release.get("tag_name") == tag]
        if len(matches) > 1:
            raise ValueError(f"multiple releases found for {tag}")
        return matches[0] if matches else None

    def create(self, tag, notes_file):
        self.run("release", "create", tag, "--draft", "--verify-tag", "--notes-file", str(notes_file), "--title", f"流光 TV {tag}")

    def upload(self, tag, file):
        self.run("release", "upload", tag, str(file), "--clobber")

    def publish(self, tag, notes_file):
        self.run("release", "edit", tag, "--draft=false", "--latest", "--notes-file", str(notes_file))


def prepare_release(github, tag, changelog):
    notes = validate_changelog(release_tag(tag), changelog)
    if github.find(tag) is not None:
        return  # Reuse drafts and published releases; build uploads replace assets.
    with tempfile.TemporaryDirectory() as directory:
        path = Path(directory) / "notes.md"
        path.write_text(notes, encoding="utf-8")
        github.create(tag, path)


def publish_release(github, tag, directory, notes_file):
    release_tag(tag)
    if not notes_file.read_text(encoding="utf-8").strip():
        raise ValueError("release notes must not be empty")
    checksum = directory / "SHA256SUMS.txt"
    if not checksum.is_file() or checksum.is_symlink() or checksum.stat().st_size == 0:
        raise ValueError("missing combined release checksums")
    # Platform jobs have already uploaded their packages and optional deltas. Their checksum
    # files share a name, so only the final combined checksum is uploaded here.
    github.upload(tag, checksum)
    github.publish(tag, notes_file)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    draft = commands.add_parser("draft")
    draft.add_argument("tag")
    draft.add_argument("changelog", type=Path)
    publish = commands.add_parser("publish")
    publish.add_argument("tag")
    publish.add_argument("directory", type=Path)
    publish.add_argument("notes_file", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "draft":
            prepare_release(GitHub(), args.tag, args.changelog)
        else:
            publish_release(GitHub(), args.tag, args.directory, args.notes_file)
    except (OSError, ValueError, RuntimeError) as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
