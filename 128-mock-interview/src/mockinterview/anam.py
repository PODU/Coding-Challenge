"""Anam session tokens.

The API key never leaves the server; the browser only gets a short-lived
session token. `MockAnamClient` lets the app run without an Anam account.
"""

from __future__ import annotations

import uuid
from typing import Protocol

import httpx

from .config import ANAM_SESSION_TOKEN_URL


class AnamError(Exception):
    """Raised when the Anam API rejects a request or is unreachable."""


class AnamClient(Protocol):
    def create_session_token(self, persona_config: dict) -> str: ...


class HttpAnamClient:
    def __init__(self, api_key: str, timeout: float = 15.0):
        self._api_key = api_key
        self._timeout = timeout

    def create_session_token(self, persona_config: dict) -> str:
        try:
            resp = httpx.post(
                ANAM_SESSION_TOKEN_URL,
                json={"personaConfig": persona_config},
                headers={
                    "Authorization": f"Bearer {self._api_key}",
                    "Content-Type": "application/json",
                },
                timeout=self._timeout,
            )
        except httpx.HTTPError as exc:
            raise AnamError(f"Could not reach Anam API: {exc}") from exc
        if resp.status_code == 401:
            raise AnamError("Anam API rejected the API key (401). Check ANAM_API_KEY.")
        if resp.status_code >= 400:
            raise AnamError(f"Anam API error {resp.status_code}: {resp.text[:300]}")
        token = resp.json().get("sessionToken")
        if not token:
            raise AnamError("Anam API response did not contain a sessionToken.")
        return token


class MockAnamClient:
    """Returns fake tokens and records each request, for tests."""

    def __init__(self):
        self.requests: list[dict] = []

    def create_session_token(self, persona_config: dict) -> str:
        self.requests.append(persona_config)
        return f"mock-session-token-{uuid.uuid4().hex[:8]}"
