"""Permission policy for ``run_command``.

Modes: ``deny`` never runs commands, ``prompt`` (the default) asks the user
each time, ``allow`` runs everything. Tests pass their own confirmer instead
of reading stdin.
"""

from __future__ import annotations

from typing import Callable

VALID_MODES = ("allow", "prompt", "deny")

Confirmer = Callable[[str], bool]


def _default_confirmer(command: str) -> bool:
    try:
        answer = input(f"Run command in workspace? {command!r} [y/N] ")
    except EOFError:
        return False
    return answer.strip().lower() in ("y", "yes")


class PermissionPolicy:
    """Decides whether a shell command may run."""

    def __init__(
        self,
        mode: str = "prompt",
        confirmer: Confirmer | None = None,
    ) -> None:
        if mode not in VALID_MODES:
            raise ValueError(
                f"invalid permission mode {mode!r}; expected one of {VALID_MODES}"
            )
        self.mode = mode
        self._confirmer = confirmer or _default_confirmer

    def authorize(self, command: str) -> bool:
        if self.mode == "deny":
            return False
        if self.mode == "allow":
            return True
        return bool(self._confirmer(command))
