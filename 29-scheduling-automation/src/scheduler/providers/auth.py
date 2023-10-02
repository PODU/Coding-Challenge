"""CalDAV credentials.

* Basic auth with an app password, for iCloud, Fastmail, Nextcloud and the
  like.
* OAuth 2.0 bearer tokens, e.g. for Google. Only the refresh step is handled
  here; getting the initial token is up to the user.
"""

from __future__ import annotations

import json
import logging
import time
from dataclasses import dataclass

logger = logging.getLogger(__name__)


class AuthError(Exception):
    """Authentication or token refresh failed."""


@dataclass(frozen=True)
class BasicCredentials:
    username: str
    password: str


@dataclass
class OAuthToken:
    """An OAuth 2.0 token that can refresh itself."""

    access_token: str
    refresh_token: str
    token_uri: str
    client_id: str
    client_secret: str
    expiry: float = 0.0  # Unix seconds

    @classmethod
    def from_json(cls, raw: str) -> OAuthToken:
        """Load a token from JSON such as ``OAUTH_TOKEN_JSON``."""
        data = json.loads(raw)
        return cls(
            access_token=data["access_token"],
            refresh_token=data["refresh_token"],
            token_uri=data["token_uri"],
            client_id=data["client_id"],
            client_secret=data["client_secret"],
            expiry=float(data.get("expiry", 0.0)),
        )

    def is_expired(self, now: float | None = None) -> bool:
        """True if expired or expiring within the next 60 seconds."""
        now = time.time() if now is None else now
        return now >= (self.expiry - 60)

    def refresh(self, *, http_post=None, now: float | None = None) -> str:
        """Get a new access token and return it. Raises AuthError on failure.

        *http_post* defaults to ``requests.post``; tests pass a fake.
        """
        if http_post is None:  # pragma: no cover - exercised via injection
            import requests

            http_post = requests.post

        try:
            response = http_post(
                self.token_uri,
                data={
                    "grant_type": "refresh_token",
                    "refresh_token": self.refresh_token,
                    "client_id": self.client_id,
                    "client_secret": self.client_secret,
                },
                timeout=30,
            )
        except Exception as exc:  # noqa: BLE001
            raise AuthError(f"Token refresh request failed: {exc}") from exc

        if getattr(response, "status_code", 500) >= 400:
            raise AuthError(f"Token refresh rejected: HTTP {response.status_code}")

        payload = response.json()
        self.access_token = payload["access_token"]
        clock = time.time() if now is None else now
        self.expiry = clock + float(payload.get("expires_in", 3600))
        return self.access_token

    def authorization_header(self, *, http_post=None) -> str:
        """The ``Authorization`` header value, refreshing the token if needed."""
        if self.is_expired():
            logger.info("OAuth access token expired; refreshing")
            self.refresh(http_post=http_post)
        return f"Bearer {self.access_token}"
