"""The workspace sandbox.

Paths from the model are untrusted. File tools must go through
:meth:`Workspace.resolve`, which resolves ``..`` and symlinks and rejects
anything that ends up outside the root.
"""

from __future__ import annotations

from pathlib import Path

from agent.errors import SandboxViolation


class Workspace:
    def __init__(self, root: str | Path) -> None:
        # Resolve once so later containment checks compare canonical paths.
        self.root = Path(root).resolve(strict=True)
        if not self.root.is_dir():
            raise ValueError(f"workspace root is not a directory: {self.root}")

    def resolve(self, rel: str) -> Path:
        """Resolve ``rel`` against the root, raising :class:`SandboxViolation` if
        the result is outside it (``..``, absolute paths, symlinks)."""
        if rel is None:
            raise SandboxViolation("path is required")
        candidate = (self.root / rel).resolve()
        if not candidate.is_relative_to(self.root):
            raise SandboxViolation(f"path escapes workspace: {rel!r}")
        return candidate

    def __repr__(self) -> str:  # pragma: no cover - debug aid
        return f"Workspace(root={self.root!r})"
