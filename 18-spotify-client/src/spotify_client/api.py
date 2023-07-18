"""A small wrapper over the Spotify Web API that returns dataclasses."""

from __future__ import annotations

import time
from typing import Any, Dict, List, Optional

import requests

from .auth import AuthManager
from .config import API_BASE
from .errors import (
    ApiError,
    AuthError,
    NoActiveDeviceError,
    PremiumRequiredError,
    RateLimited,
)
from .models import Device, Playback, Playlist, Track

DEFAULT_TIMEOUT = 15
MAX_PAGE_LIMIT = 50  # Spotify's per-request cap for these endpoints.


class SpotifyClient:
    """Refreshes the token when needed and retries once on 401 and once on
    429 (after waiting for Retry-After)."""

    def __init__(self, auth: AuthManager,
                 session: Optional[requests.Session] = None,
                 sleep=time.sleep) -> None:
        self.auth = auth
        self.session = session or auth.session
        self._sleep = sleep

    def _request(self, method: str, path: str, *,
                 params: Optional[Dict[str, Any]] = None,
                 json: Optional[Dict[str, Any]] = None,
                 _retried_auth: bool = False,
                 _retried_rate: bool = False) -> Optional[Dict[str, Any]]:
        """Send an authenticated request and return the JSON body, or ``None``
        for an empty response. Errors become the exceptions in errors.py."""
        url = path if path.startswith("http") else f"{API_BASE}{path}"
        headers = {"Authorization": f"Bearer {self.auth.valid_access_token()}"}

        try:
            resp = self.session.request(
                method, url, headers=headers, params=params, json=json,
                timeout=DEFAULT_TIMEOUT,
            )
        except requests.RequestException as exc:
            raise ApiError(f"Network error calling Spotify: {exc}") from exc

        status = resp.status_code

        if status == 401 and not _retried_auth:
            # The token can be revoked before it expires; refresh and retry once.
            self.auth.refresh()
            return self._request(method, path, params=params, json=json,
                                 _retried_auth=True, _retried_rate=_retried_rate)

        if status == 429 and not _retried_rate:
            retry_after = int(resp.headers.get("Retry-After", "1") or "1")
            self._sleep(retry_after)
            return self._request(method, path, params=params, json=json,
                                 _retried_auth=_retried_auth, _retried_rate=True)
        if status == 429:
            retry_after = int(resp.headers.get("Retry-After", "0") or "0")
            raise RateLimited("Rate limited by Spotify; try again later.", retry_after)

        if status == 403:
            detail = _error_message(resp)
            if "premium" in detail.lower():
                raise PremiumRequiredError(
                    "This action requires Spotify Premium."
                )
            raise ApiError(f"Forbidden: {detail}", status)

        if status == 404:
            detail = _error_message(resp)
            if "device" in detail.lower():
                raise NoActiveDeviceError(_NO_DEVICE_MSG)
            raise ApiError(f"Not found: {detail}", status)

        if status == 204 or not resp.content:
            return None

        if not (200 <= status < 300):
            raise ApiError(f"Spotify API error ({status}): {_error_message(resp)}", status)

        try:
            return resp.json()
        except ValueError:
            return None

    def current_user(self) -> Dict[str, Any]:
        """Return the raw ``GET /v1/me`` profile."""
        data = self._request("GET", "/me")
        if not data:
            raise AuthError("Empty profile response from Spotify.")
        return data

    def playlists(self, limit: int = 50) -> List[Playlist]:
        return [Playlist.from_dict(item)
                for item in self._paginate("/me/playlists", limit)]

    def search_tracks(self, query: str, limit: int = 20) -> List[Track]:
        items = self._paginate(
            "/search", limit,
            params={"q": query, "type": "track"},
            container="tracks",
        )
        return [Track.from_dict(item) for item in items]

    def _paginate(self, path: str, limit: int, *,
                  params: Optional[Dict[str, Any]] = None,
                  container: Optional[str] = None) -> List[Dict[str, Any]]:
        """Collect up to ``limit`` items by following ``next`` links.

        ``container`` names the key the page is nested under, e.g. search
        results arrive as ``{"tracks": {...}}``.
        """
        collected: List[Dict[str, Any]] = []
        page_params = dict(params or {})
        page_params["limit"] = min(limit, MAX_PAGE_LIMIT)
        url: Optional[str] = path
        next_params: Optional[Dict[str, Any]] = page_params

        while url and len(collected) < limit:
            data = self._request("GET", url, params=next_params) or {}
            page = data.get(container, data) if container else data
            items = page.get("items", []) if isinstance(page, dict) else []
            collected.extend(items)
            url = page.get("next") if isinstance(page, dict) else None
            next_params = None  # ``next`` already carries the query string.

        return collected[:limit]

    def devices(self) -> List[Device]:
        data = self._request("GET", "/me/player/devices") or {}
        return [Device.from_dict(d) for d in data.get("devices", [])]

    def current_playback(self) -> Playback:
        """Raises NoActiveDeviceError if nothing is playing anywhere (Spotify
        answers 204)."""
        data = self._request("GET", "/me/player")
        if not data:
            raise NoActiveDeviceError(_NO_DEVICE_MSG)
        return Playback.from_dict(data)

    def play(self, *, context_uri: Optional[str] = None,
             uris: Optional[List[str]] = None,
             device_id: Optional[str] = None) -> None:
        body: Dict[str, Any] = {}
        if context_uri:
            body["context_uri"] = context_uri
        if uris:
            body["uris"] = uris
        params = {"device_id": device_id} if device_id else None
        self._request("PUT", "/me/player/play", params=params,
                      json=body or None)

    def pause(self, *, device_id: Optional[str] = None) -> None:
        params = {"device_id": device_id} if device_id else None
        self._request("PUT", "/me/player/pause", params=params)

    def next(self, *, device_id: Optional[str] = None) -> None:
        params = {"device_id": device_id} if device_id else None
        self._request("POST", "/me/player/next", params=params)

    def previous(self, *, device_id: Optional[str] = None) -> None:
        params = {"device_id": device_id} if device_id else None
        self._request("POST", "/me/player/previous", params=params)

    def transfer(self, device_id: str, *, play: bool = True) -> None:
        self._request("PUT", "/me/player",
                      json={"device_ids": [device_id], "play": play})

    def resolve_device(self, selector: str) -> Device:
        """Find a device by exact id, else by case-insensitive name."""
        devices = self.devices()
        for d in devices:
            if d.id == selector:
                return d
        for d in devices:
            if d.name.lower() == selector.lower():
                return d
        raise ApiError(f"No device matching '{selector}'. Run `spotify devices`.")


_NO_DEVICE_MSG = (
    "No active device. Open Spotify on a device, or run `spotify devices` then "
    "`spotify transfer <device>`."
)


def _error_message(resp: requests.Response) -> str:
    try:
        body = resp.json()
    except ValueError:
        return resp.text[:200] if resp.text else f"HTTP {resp.status_code}"
    err = body.get("error")
    if isinstance(err, dict):
        return err.get("message") or str(err)
    return str(err or body)


__all__ = ["SpotifyClient"]
