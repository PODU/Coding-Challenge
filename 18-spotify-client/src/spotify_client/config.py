"""Configuration, read from environment variables (or a .env file)."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import List, Optional

from .errors import ConfigError

SCOPES: List[str] = [
    "user-read-private",
    "user-read-email",
    "playlist-read-private",
    "user-read-playback-state",
    "user-modify-playback-state",
    "user-read-currently-playing",
]

DEFAULT_REDIRECT_URI = "http://127.0.0.1:8888/callback"

AUTHORIZE_URL = "https://accounts.spotify.com/authorize"
TOKEN_URL = "https://accounts.spotify.com/api/token"
API_BASE = "https://api.spotify.com/v1"


def _maybe_load_dotenv() -> None:
    """Load ``.env`` if python-dotenv happens to be installed."""
    try:
        from dotenv import load_dotenv  # type: ignore[import-not-found]
    except Exception:  # pragma: no cover - optional dependency
        return
    load_dotenv()


@dataclass
class Config:
    """``client_id`` is not a secret: PKCE needs no client secret, and the id
    appears in the authorize URL anyway."""

    client_id: str
    redirect_uri: str = DEFAULT_REDIRECT_URI
    scopes: List[str] = field(default_factory=lambda: list(SCOPES))

    @property
    def scope_string(self) -> str:
        return " ".join(self.scopes)

    @classmethod
    def from_env(cls, env: Optional[dict] = None) -> "Config":
        """Read ``SPOTIFY_CLIENT_ID`` (required) and ``SPOTIFY_REDIRECT_URI``."""
        if env is None:
            _maybe_load_dotenv()
            env = dict(os.environ)

        client_id = (env.get("SPOTIFY_CLIENT_ID") or "").strip()
        if not client_id:
            raise ConfigError(
                "SPOTIFY_CLIENT_ID is not set. Copy .env.example to .env and "
                "fill in the Client ID from https://developer.spotify.com/dashboard, "
                "or export SPOTIFY_CLIENT_ID in your shell."
            )

        redirect_uri = (env.get("SPOTIFY_REDIRECT_URI") or DEFAULT_REDIRECT_URI).strip()
        return cls(client_id=client_id, redirect_uri=redirect_uri)


__all__ = [
    "Config",
    "SCOPES",
    "DEFAULT_REDIRECT_URI",
    "AUTHORIZE_URL",
    "TOKEN_URL",
    "API_BASE",
]
