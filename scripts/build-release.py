#!/usr/bin/env python3
"""Cross-compile chromium-bookmarks-mcp binaries for GitHub Releases."""
import argparse
import concurrent.futures
import hashlib
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent
TARGETS = [
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
]


def run(args, **kwargs):
    subprocess.run(args, cwd=ROOT, check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    dist = ROOT / "dist"
    if dist.exists():
        shutil.rmtree(dist)
    dist.mkdir()

    ldflags = (
        f"-s -w -X github.com/chiehw/chromium-bookmarks-mcp/internal/version.Version={args.version}"
    )
    jobs = []
    for goos, arch in TARGETS:
        suffix = ".exe" if goos == "windows" else ""
        jobs.append((goos, arch, dist / f"chromium-bookmarks-mcp-{goos}-{arch}{suffix}"))

    def build(job):
        goos, arch, target = job
        env = dict(os.environ, CGO_ENABLED="0", GOOS=goos, GOARCH=arch)
        env.pop("GOARM", None)
        print(f"Building {target.name}", flush=True)
        run(
            [
                "go",
                "build",
                "-trimpath",
                "-ldflags",
                ldflags,
                "-o",
                str(target),
                "./cmd/chromium-bookmarks-mcp",
            ],
            env=env,
        )
        print(f"Built {target.name}", flush=True)

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        list(executor.map(build, jobs))

    files = sorted(path for path in dist.iterdir() if path.is_file() and path.name != "SHA256SUMS")
    with (dist / "SHA256SUMS").open("w") as checksums:
        for path in files:
            with path.open("rb") as source:
                digest = hashlib.file_digest(source, "sha256").hexdigest()
            checksums.write(f"{digest}  {path.name}\n")
    print(f"Release {args.version}: {len(files)} assets plus SHA256SUMS", flush=True)


if __name__ == "__main__":
    main()
