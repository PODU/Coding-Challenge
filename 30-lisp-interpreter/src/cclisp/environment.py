"""Variable scopes."""

from __future__ import annotations

from typing import Iterable

from .types import LispNameError


class Environment(dict):
    """Bindings for one scope, with a link to the enclosing one."""

    def __init__(
        self,
        params: Iterable[str] = (),
        args: Iterable[object] = (),
        parent: "Environment | None" = None,
    ) -> None:
        super().__init__()
        self.update(zip(params, args))
        self.parent = parent

    def find(self, var: str) -> "Environment":
        """Return the innermost scope that binds *var*."""
        env: "Environment | None" = self
        while env is not None:
            if var in env:
                return env
            env = env.parent
        raise LispNameError(f"unbound symbol: {var}")
