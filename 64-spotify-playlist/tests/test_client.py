"""Client tests: pagination, 401 refresh, 429 backoff, bearer injection."""

from __future__ import annotations

import pytest
import responses

from spotify_playlist.client import SpotifyClient, SpotifyApiError


class FakeProvider:
    """A token provider whose token changes on refresh()."""

    def __init__(self):
        self.token = "TOKEN-1"
        self.refresh_calls = 0

    def get_token(self) -> str:
        return self.token

    def refresh(self) -> str:
        self.refresh_calls += 1
        self.token = f"TOKEN-{self.refresh_calls + 1}"
        return self.token


@pytest.fixture
def provider():
    return FakeProvider()


@responses.activate
def test_bearer_header_injected(provider):
    responses.get("https://api.spotify.com/v1/me", json={"id": "alice"}, status=200)
    client = SpotifyClient(provider)
    assert client.get("/v1/me") == {"id": "alice"}
    assert responses.calls[0].request.headers["Authorization"] == "Bearer TOKEN-1"


@responses.activate
def test_pagination_follows_next(provider, tracks_page_1, tracks_page_2):
    responses.get(
        "https://api.spotify.com/v1/playlists/PL123/tracks",
        json=tracks_page_1,
        status=200,
    )
    responses.get(
        "https://api.spotify.com/v1/playlists/PL123/tracks",
        json=tracks_page_2,
        status=200,
        match=[responses.matchers.query_param_matcher({"offset": "100", "limit": "100"})],
    )
    client = SpotifyClient(provider)
    items = list(
        client.get_paginated("/v1/playlists/PL123/tracks", params={"limit": 100})
    )
    # 4 items on page 1 + 1 on page 2 = 5 raw items yielded.
    assert len(items) == 5
    assert len(responses.calls) == 2


@responses.activate
def test_401_triggers_single_refresh_then_retry(provider):
    responses.get("https://api.spotify.com/v1/me", json={}, status=401)
    responses.get("https://api.spotify.com/v1/me", json={"id": "alice"}, status=200)

    client = SpotifyClient(provider)
    assert client.get("/v1/me") == {"id": "alice"}
    assert provider.refresh_calls == 1
    # Second request used the refreshed token.
    assert responses.calls[1].request.headers["Authorization"] == "Bearer TOKEN-2"


@responses.activate
def test_401_twice_raises(provider):
    responses.get("https://api.spotify.com/v1/me", json={}, status=401)
    responses.get("https://api.spotify.com/v1/me", json={}, status=401)
    client = SpotifyClient(provider)
    with pytest.raises(SpotifyApiError) as exc:
        client.get("/v1/me")
    assert exc.value.status_code == 401
    assert provider.refresh_calls == 1


@responses.activate
def test_429_honors_retry_after(provider):
    slept = []
    responses.get(
        "https://api.spotify.com/v1/me",
        json={},
        status=429,
        headers={"Retry-After": "7"},
    )
    responses.get("https://api.spotify.com/v1/me", json={"ok": True}, status=200)

    client = SpotifyClient(provider, sleep_func=slept.append)
    assert client.get("/v1/me") == {"ok": True}
    assert slept == [7.0]


@responses.activate
def test_429_exhausts_retries(provider):
    for _ in range(6):
        responses.get(
            "https://api.spotify.com/v1/me",
            json={},
            status=429,
            headers={"Retry-After": "0"},
        )
    client = SpotifyClient(provider, max_retries=3, sleep_func=lambda s: None)
    with pytest.raises(SpotifyApiError) as exc:
        client.get("/v1/me")
    assert exc.value.status_code == 429


@responses.activate
def test_backoff_without_retry_after(provider):
    slept = []
    responses.get("https://api.spotify.com/v1/me", json={}, status=429)
    responses.get("https://api.spotify.com/v1/me", json={"ok": True}, status=200)
    client = SpotifyClient(provider, sleep_func=slept.append)
    client.get("/v1/me")
    assert slept == [1.0]  # 2 ** 0
