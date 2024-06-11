"""Configuration. Credentials come from the environment, which can be filled
in from a ``.env`` file."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Mapping, Optional

DEFAULT_TOKEN_CACHE = "~/.spotify_playlist_token.json"

# Scopes requested when the user logs in.
USER_SCOPES = (
    "playlist-read-private",
    "playlist-read-collaborative",
    "playlist-modify-private",
    "playlist-modify-public",
)


class ConfigError(RuntimeError):
    """Required configuration is missing or invalid."""


def load_dotenv(path: str | os.PathLike[str] = ".env") -> None:
    """Load ``KEY=VALUE`` lines from a ``.env`` file, if there is one.

    Variables already set in the environment win.
    """
    p = Path(path)
    if not p.is_file():
        return
    for raw in p.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key and key not in os.environ:
            os.environ[key] = value


@dataclass(frozen=True)
class Config:
    client_id: str
    client_secret: str
    redirect_uri: Optional[str] = None
    token_cache: str = DEFAULT_TOKEN_CACHE

    @property
    def token_cache_path(self) -> Path:
        return Path(os.path.expanduser(self.token_cache))

    @classmethod
    def from_env(
        cls,
        environ: Optional[Mapping[str, str]] = None,
        *,
        require_redirect: bool = False,
    ) -> "Config":
        """Read the configuration from the environment, raising
        :class:`ConfigError` if something required is missing."""
        env = os.environ if environ is None else environ

        client_id = (env.get("SPOTIFY_CLIENT_ID") or "").strip()
        client_secret = (env.get("SPOTIFY_CLIENT_SECRET") or "").strip()
        redirect_uri = (env.get("SPOTIFY_REDIRECT_URI") or "").strip() or None
        token_cache = (env.get("SPOTIFY_TOKEN_CACHE") or "").strip() or DEFAULT_TOKEN_CACHE

        missing = []
        if not client_id:
            missing.append("SPOTIFY_CLIENT_ID")
        if not client_secret:
            missing.append("SPOTIFY_CLIENT_SECRET")
        if require_redirect and not redirect_uri:
            missing.append("SPOTIFY_REDIRECT_URI")
        if missing:
            raise ConfigError(
                "Missing required environment variable(s): "
                + ", ".join(missing)
                + ". Copy .env.example to .env and fill in your Spotify app credentials "
                "(https://developer.spotify.com/dashboard)."
            )

        return cls(
            client_id=client_id,
            client_secret=client_secret,
            redirect_uri=redirect_uri,
            token_cache=token_cache,
        )
