"""A small Spotify Web API client that handles auth, paging and rate limits."""

from __future__ import annotations

import logging
import time
from typing import Any, Callable, Dict, Iterator, Optional
from urllib.parse import urljoin

import requests

from .auth import TokenProvider

log = logging.getLogger(__name__)

API_BASE = "https://api.spotify.com"
DEFAULT_MAX_RETRIES = 5
# Longest backoff when a 429 comes without a Retry-After header.
BACKOFF_CAP_SECONDS = 60


class SpotifyApiError(RuntimeError):
    """An API error that retrying won't fix."""

    def __init__(self, status_code: int, message: str) -> None:
        super().__init__(f"Spotify API error {status_code}: {message}")
        self.status_code = status_code
        self.message = message


class SpotifyClient:
    def __init__(
        self,
        token_provider: TokenProvider,
        *,
        session: Optional[requests.Session] = None,
        base_url: str = API_BASE,
        max_retries: int = DEFAULT_MAX_RETRIES,
        sleep_func: Callable[[float], None] = time.sleep,
    ) -> None:
        self._tokens = token_provider
        self._session = session or requests.Session()
        self._base_url = base_url.rstrip("/")
        self._max_retries = max_retries
        self._sleep = sleep_func

    def _full_url(self, path_or_url: str) -> str:
        if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
            return path_or_url
        return f"{self._base_url}/{path_or_url.lstrip('/')}"

    def request(
        self,
        method: str,
        path_or_url: str,
        *,
        params: Optional[Dict[str, Any]] = None,
        json: Optional[Any] = None,
    ) -> requests.Response:
        """Send a request. On a 401 the token is refreshed and the request
        retried once; on a 429 we wait and retry."""
        url = self._full_url(path_or_url)
        attempts = 0
        refreshed = False

        while True:
            token = self._tokens.get_token()
            resp = self._session.request(
                method,
                url,
                params=params,
                json=json,
                headers={"Authorization": f"Bearer {token}"},
            )

            if resp.status_code == 401 and not refreshed:
                log.info("Got 401; refreshing token and retrying once")
                self._tokens.refresh()
                refreshed = True
                continue

            if resp.status_code == 429:
                if attempts >= self._max_retries:
                    raise SpotifyApiError(429, "Rate limited; max retries exceeded")
                delay = self._retry_delay(resp, attempts)
                log.warning("Rate limited (429); sleeping %.1fs then retrying", delay)
                self._sleep(delay)
                attempts += 1
                continue

            if resp.status_code >= 400:
                raise SpotifyApiError(resp.status_code, resp.text)

            return resp

    def _retry_delay(self, resp: requests.Response, attempt: int) -> float:
        retry_after = resp.headers.get("Retry-After")
        if retry_after is not None:
            try:
                return float(retry_after)
            except ValueError:
                pass
        return float(min(BACKOFF_CAP_SECONDS, 2 ** attempt))

    def get(self, path_or_url: str, *, params: Optional[Dict[str, Any]] = None) -> Any:
        return self.request("GET", path_or_url, params=params).json()

    def post(self, path_or_url: str, *, json: Optional[Any] = None) -> Any:
        resp = self.request("POST", path_or_url, json=json)
        if not resp.content:
            return {}
        try:
            return resp.json()
        except ValueError:
            return {}

    def get_paginated(
        self,
        path_or_url: str,
        *,
        params: Optional[Dict[str, Any]] = None,
    ) -> Iterator[Any]:
        """Yield the items from every page, following ``next`` links.

        Also handles pages wrapped in another object, as search results are
        (``{"tracks": {"items": ..., "next": ...}}``).
        """
        next_url: Optional[str] = self._full_url(path_or_url)
        first = True
        while next_url:
            # The next link already includes the query string.
            page = _unwrap_paging(self.get(next_url, params=params if first else None))
            yield from page.get("items", [])
            next_url = page.get("next")
            first = False


def _unwrap_paging(page: Any) -> Dict[str, Any]:
    """Return the paging object, looking one level down if needed."""
    if isinstance(page, dict) and "items" not in page:
        for value in page.values():
            if isinstance(value, dict) and "items" in value:
                return value
    return page if isinstance(page, dict) else {}
