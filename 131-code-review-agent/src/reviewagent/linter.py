"""Running ruff on a checkout of the PR and feeding its findings to the model.

Parsing is kept apart from running ruff, so the tests don't need ruff.
"""

from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass


@dataclass
class LintFinding:
    file: str
    line: int
    code: str
    message: str


def parse_ruff_json(output: str) -> list[LintFinding]:
    if not output.strip():
        return []
    return [
        LintFinding(
            file=item.get("filename", ""),
            line=item.get("location", {}).get("row", 0),
            code=item.get("code") or "",
            message=item.get("message", ""),
        )
        for item in json.loads(output)
    ]


def run_ruff(repo_dir: str, *, runner=subprocess.run) -> list[LintFinding]:
    """Run ruff in ``repo_dir``.

    With --exit-zero ruff exits 0 even when it finds problems, so any other
    exit code means ruff itself failed.
    """
    proc = runner(
        ["ruff", "check", ".", "--output-format", "json", "--exit-zero"],
        cwd=repo_dir,
        capture_output=True,
        text=True,
        timeout=300,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"ruff failed ({proc.returncode}): {proc.stderr[:500]}")
    return parse_ruff_json(proc.stdout)


def format_findings(findings: list[LintFinding], *, repo_prefix: str = "") -> str:
    """Format findings as a list of ``path:line [code] message`` lines."""
    if not findings:
        return ""
    lines = []
    for f in findings:
        path = f.file
        if repo_prefix and path.startswith(repo_prefix):
            path = path[len(repo_prefix):].lstrip("/\\")
        lines.append(f"- {path}:{f.line} [{f.code}] {f.message}")
    return "\n".join(lines)


def clone_at_commit(clone_url: str, sha: str, dest: str, *, runner=subprocess.run) -> None:
    """Fetch just the one commit, without history."""
    for cmd in (
        ["git", "init", "-q", dest],
        ["git", "-C", dest, "remote", "add", "origin", clone_url],
        ["git", "-C", dest, "fetch", "-q", "--depth", "1", "origin", sha],
        ["git", "-C", dest, "checkout", "-q", "FETCH_HEAD"],
    ):
        proc = runner(cmd, capture_output=True, text=True, timeout=300)
        if proc.returncode != 0:
            raise RuntimeError(f"{' '.join(cmd)} failed: {proc.stderr[:500]}")
