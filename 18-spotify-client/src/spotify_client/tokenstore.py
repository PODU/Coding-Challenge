"""Stores OAuth tokens in the user's config directory, readable only by them."""

from __future__ import annotations

import json
import os
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Dict, Optional

# Treat tokens as expired this long before they really are, so a token doesn't
# run out between our check and Spotify's.
EXPIRY_SKEW_SECONDS = 60


def default_token_path() -> Path:
    """``%APPDATA%`` on Windows, ``$XDG_CONFIG_HOME`` (or ~/.config) elsewhere."""
    if os.name == "nt":
        base = os.environ.get("APPDATA") or str(Path.home() / "AppData" / "Roaming")
    else:
        base = os.environ.get("XDG_CONFIG_HOME") or str(Path.home() / ".config")
    return Path(base) / "spotify-client" / "tokens.json"


@dataclass
class Token:
    """``expires_at`` is a Unix timestamp."""

    access_token: str
    refresh_token: Optional[str] = None
    expires_at: float = 0.0
    token_type: str = "Bearer"
    scope: str = ""

    @classmethod
    def from_response(cls, data: Dict[str, Any], *, now: Optional[float] = None,
                      previous: Optional["Token"] = None) -> "Token":
        """Build a token from the token endpoint's response.

        Refresh responses may leave out ``refresh_token``, in which case the
        previous one stays valid and is kept.
        """
        now = time.time() if now is None else now
        expires_in = int(data.get("expires_in") or 0)
        refresh = data.get("refresh_token")
        if not refresh and previous is not None:
            refresh = previous.refresh_token
        return cls(
            access_token=data["access_token"],
            refresh_token=refresh,
            expires_at=now + expires_in,
            token_type=data.get("token_type", "Bearer"),
            scope=data.get("scope", previous.scope if previous else ""),
        )

    def is_expired(self, *, skew: int = EXPIRY_SKEW_SECONDS,
                   now: Optional[float] = None) -> bool:
        now = time.time() if now is None else now
        return now >= (self.expires_at - skew)

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Token":
        """Unknown keys are ignored."""
        return cls(
            access_token=data["access_token"],
            refresh_token=data.get("refresh_token"),
            expires_at=float(data.get("expires_at") or 0.0),
            token_type=data.get("token_type", "Bearer"),
            scope=data.get("scope", ""),
        )


class TokenStore:
    def __init__(self, path: Optional[Path] = None) -> None:
        self.path = Path(path) if path is not None else default_token_path()

    def load(self) -> Optional[Token]:
        """Return the stored token, or ``None`` if missing or unreadable."""
        try:
            raw = self.path.read_text(encoding="utf-8")
        except OSError:
            return None
        try:
            return Token.from_dict(json.loads(raw))
        except (json.JSONDecodeError, KeyError):
            return None

    def save(self, token: Token) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        data = json.dumps(token.to_dict(), indent=2)
        # Create the file as 0600 so it's never readable by others, even briefly.
        fd = os.open(str(self.path), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as fh:
                fh.write(data)
        finally:
            try:
                os.chmod(self.path, 0o600)
            except OSError:  # pragma: no cover
                pass

    def clear(self) -> None:
        try:
            self.path.unlink()
        except FileNotFoundError:
            pass


__all__ = ["Token", "TokenStore", "default_token_path", "EXPIRY_SKEW_SECONDS"]
