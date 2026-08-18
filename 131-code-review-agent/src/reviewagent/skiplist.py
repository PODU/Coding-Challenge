"""Deciding which files to send to the model.

Generated, vendored and lock files only waste tokens and add noise.
"""

from __future__ import annotations

import fnmatch

DEFAULT_SKIP_PATTERNS = [
    "*.lock",
    "uv.lock",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "Cargo.lock",
    "poetry.lock",
    "Gemfile.lock",
    "composer.lock",
    "go.sum",
    "*.min.js",
    "*.min.css",
    "*.map",
    "*.svg",
    "*.png",
    "*.jpg",
    "*.gif",
    "*.ico",
    "*.pdf",
    "*.snap",
    "vendor/*",
    "node_modules/*",
    "dist/*",
    "build/*",
    "*.pb.go",
    "*_pb2.py",
    "*.generated.*",
]


def skip_reason(filename: str, patterns: list[str] | None = None) -> str | None:
    """Return why the file should be skipped, or None to review it."""
    name = filename.replace("\\", "/")
    basename = name.rsplit("/", 1)[-1]
    for pattern in patterns or DEFAULT_SKIP_PATTERNS:
        if "/" in pattern:
            # Directory patterns can match at any depth.
            if fnmatch.fnmatch(name, pattern) or fnmatch.fnmatch(name, f"*/{pattern}"):
                return f"matches vendored/generated path pattern '{pattern}'"
        elif fnmatch.fnmatch(basename, pattern):
            return f"matches generated/lock file pattern '{pattern}'"
    return None


def partition(files, patterns: list[str] | None = None):
    """Split files into (to_review, skipped), with skipped holding
    (file, reason) pairs."""
    to_review, skipped = [], []
    for f in files:
        reason = skip_reason(f.filename, patterns)
        if reason is None and not f.patch:
            reason = "no textual diff available (binary or oversized file)"
        if reason:
            skipped.append((f, reason))
        else:
            to_review.append(f)
    return to_review, skipped
