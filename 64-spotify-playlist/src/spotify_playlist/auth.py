"""Spotify OAuth.

Client Credentials is enough to read public playlists. Anything involving the
user's own or private playlists needs the Authorization Code flow with PKCE;
its refresh token is cached on disk, readable only by the user.
"""

from __future__ import annotations

import base64
import hashlib
import json
import logging
import os
import secrets
import time
from pathlib import Path
from typing import Callable, Iterable, Optional, Protocol
from urllib.parse import urlencode

import requests

from .config import Config, USER_SCOPES

log = logging.getLogger(__name__)

ACCOUNTS_BASE = "https://accounts.spotify.com"
TOKEN_URL = f"{ACCOUNTS_BASE}/api/token"
AUTHORIZE_URL = f"{ACCOUNTS_BASE}/authorize"

# Refresh a little early so a request doesn't race the expiry and get a 401.
EXPIRY_SKEW_SECONDS = 60


class AuthError(RuntimeError):
    """Getting a token failed."""


class TokenProvider(Protocol):
    """Supplies access tokens. The client calls refresh() after a 401."""

    def get_token(self) -> str:
        """Return a current access token, refreshing it if it has expired."""
        ...

    def refresh(self) -> str:
        """Get a new access token even if the current one looks valid."""
        ...


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def generate_pkce_pair() -> tuple[str, str]:
    """Return a PKCE ``(code_verifier, code_challenge)`` pair using S256."""
    verifier = _b64url(secrets.token_bytes(32))
    challenge = _b64url(hashlib.sha256(verifier.encode("ascii")).digest())
    return verifier, challenge


def build_authorize_url(
    config: Config,
    *,
    scopes: Iterable[str] = USER_SCOPES,
    state: str,
    code_challenge: str,
) -> str:
    """Return the URL where the user grants us access."""
    if not config.redirect_uri:
        raise AuthError("SPOTIFY_REDIRECT_URI is required for the user login flow.")
    params = {
        "response_type": "code",
        "client_id": config.client_id,
        "redirect_uri": config.redirect_uri,
        "scope": " ".join(scopes),
        "state": state,
        "code_challenge_method": "S256",
        "code_challenge": code_challenge,
    }
    return f"{AUTHORIZE_URL}?{urlencode(params)}"


def _token_headers(config: Config) -> dict[str, str]:
    raw = f"{config.client_id}:{config.client_secret}".encode("utf-8")
    return {
        "Authorization": "Basic " + base64.b64encode(raw).decode("ascii"),
        "Content-Type": "application/x-www-form-urlencoded",
    }


class TokenCache:
    """JSON file holding the tokens, readable only by the user (mode 0600)."""

    def __init__(self, path: str | os.PathLike[str]) -> None:
        self.path = Path(os.path.expanduser(str(path)))

    def load(self) -> dict:
        try:
            return json.loads(self.path.read_text(encoding="utf-8"))
        except (FileNotFoundError, ValueError):
            return {}

    def save(self, data: dict) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        # Set the mode at creation so the secret is never briefly world-readable.
        fd = os.open(str(self.path), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as fh:
                json.dump(data, fh)
        finally:
            try:
                os.chmod(self.path, 0o600)
            except OSError:  # pragma: no cover - e.g. on Windows
                pass


class ClientCredentialsTokenProvider:
    """App token, without a user. Only gives access to public data."""

    def __init__(
        self,
        config: Config,
        *,
        session: Optional[requests.Session] = None,
        time_func: Callable[[], float] = time.time,
    ) -> None:
        self._config = config
        self._session = session or requests.Session()
        self._time = time_func
        self._token: Optional[str] = None
        self._expires_at: float = 0.0

    def get_token(self) -> str:
        if self._token and self._time() < self._expires_at - EXPIRY_SKEW_SECONDS:
            return self._token
        return self.refresh()

    def refresh(self) -> str:
        log.debug("Requesting client-credentials token")
        resp = self._session.post(
            TOKEN_URL,
            data={"grant_type": "client_credentials"},
            headers=_token_headers(self._config),
        )
        if resp.status_code != 200:
            raise AuthError(
                f"Client-credentials token request failed ({resp.status_code}): {resp.text}"
            )
        payload = resp.json()
        self._token = payload["access_token"]
        self._expires_at = self._time() + int(payload.get("expires_in", 3600))
        return self._token


Authorizer = Callable[[str], str]
"""Given a consent URL, return the authorization ``code``."""


class AuthorizationCodeTokenProvider:
    """User token via Authorization Code + PKCE, cached on disk.

    ``authorizer`` performs the interactive login: it gets the consent URL and
    returns the code from the redirect. Tests pass a fake one.
    """

    def __init__(
        self,
        config: Config,
        *,
        scopes: Iterable[str] = USER_SCOPES,
        session: Optional[requests.Session] = None,
        authorizer: Optional[Authorizer] = None,
        time_func: Callable[[], float] = time.time,
    ) -> None:
        self._config = config
        self._scopes = tuple(scopes)
        self._session = session or requests.Session()
        self._authorizer = authorizer
        self._time = time_func
        self._cache = TokenCache(config.token_cache_path)

        cached = self._cache.load()
        self._access_token: Optional[str] = cached.get("access_token")
        self._refresh_token: Optional[str] = cached.get("refresh_token")
        self._expires_at: float = float(cached.get("expires_at", 0.0))

    def get_token(self) -> str:
        if self._access_token and self._time() < self._expires_at - EXPIRY_SKEW_SECONDS:
            return self._access_token
        if self._refresh_token:
            return self.refresh()
        return self._login()

    def refresh(self) -> str:
        if not self._refresh_token:
            return self._login()
        log.debug("Refreshing user access token")
        payload = self._token_request(
            {
                "grant_type": "refresh_token",
                "refresh_token": self._refresh_token,
                "client_id": self._config.client_id,
            }
        )
        self._apply_token(payload)
        return self._access_token  # type: ignore[return-value]

    def _login(self) -> str:
        if self._authorizer is None:
            raise AuthError(
                "No cached refresh token and no interactive authorizer configured. "
                "Run a user-flow command in a terminal to log in."
            )
        verifier, challenge = generate_pkce_pair()
        state = secrets.token_urlsafe(16)
        url = build_authorize_url(
            self._config, scopes=self._scopes, state=state, code_challenge=challenge
        )
        code = self._authorizer(url)
        payload = self._token_request(
            {
                "grant_type": "authorization_code",
                "code": code,
                "redirect_uri": self._config.redirect_uri,
                "client_id": self._config.client_id,
                "code_verifier": verifier,
            }
        )
        self._apply_token(payload)
        return self._access_token  # type: ignore[return-value]

    def _token_request(self, data: dict) -> dict:
        resp = self._session.post(
            TOKEN_URL,
            data=data,
            headers=_token_headers(self._config),
        )
        if resp.status_code != 200:
            raise AuthError(
                f"Token request failed ({resp.status_code}): {resp.text}"
            )
        return resp.json()

    def _apply_token(self, payload: dict) -> None:
        self._access_token = payload["access_token"]
        self._expires_at = self._time() + int(payload.get("expires_in", 3600))
        # Spotify sometimes issues a new refresh token; otherwise keep the old one.
        if payload.get("refresh_token"):
            self._refresh_token = payload["refresh_token"]
        self._cache.save(
            {
                "access_token": self._access_token,
                "refresh_token": self._refresh_token,
                "expires_at": self._expires_at,
            }
        )
