"""Shared fixtures: dummy credentials and sample Spotify API JSON.

Everything here is static / offline. No real credentials and no network.
"""

from __future__ import annotations

import pytest

from spotify_playlist.config import Config

DUMMY_ENV = {
    "SPOTIFY_CLIENT_ID": "test-client-id",
    "SPOTIFY_CLIENT_SECRET": "test-client-secret",
    "SPOTIFY_REDIRECT_URI": "http://127.0.0.1:8888/callback",
}


@pytest.fixture
def dummy_env(monkeypatch):
    for key, value in DUMMY_ENV.items():
        monkeypatch.setenv(key, value)
    return DUMMY_ENV


@pytest.fixture
def config(tmp_path):
    return Config(
        client_id="test-client-id",
        client_secret="test-client-secret",
        redirect_uri="http://127.0.0.1:8888/callback",
        token_cache=str(tmp_path / "token.json"),
    )


@pytest.fixture
def token_response():
    return {
        "access_token": "ACCESS-TOKEN-1",
        "token_type": "Bearer",
        "expires_in": 3600,
        "scope": "playlist-read-private",
    }


@pytest.fixture
def playlist_meta():
    return {
        "id": "PL123",
        "name": "My Mix",
        "description": "best of",
        "owner": {"id": "alice", "display_name": "Alice"},
        "tracks": {"total": 3},
    }


def _track_item(name, artist, album, uri, isrc=None, duration=200000):
    return {
        "added_at": "2021-01-01T00:00:00Z",
        "is_local": False,
        "track": {
            "type": "track",
            "name": name,
            "uri": uri,
            "duration_ms": duration,
            "is_local": False,
            "artists": [{"name": artist}],
            "album": {"name": album},
            "external_ids": {"isrc": isrc} if isrc else {},
        },
    }


@pytest.fixture
def tracks_page_1():
    """First page of tracks; includes one null item and one episode to skip."""
    return {
        "items": [
            _track_item("Song A", "Artist A", "Album A", "spotify:track:a", "US0000000001"),
            _track_item("Song B, with comma", 'Artist "B"', "Album B", "spotify:track:b"),
            {"added_at": "x", "is_local": False, "track": None},  # null -> skipped
            {
                "added_at": "x",
                "is_local": False,
                "track": {"type": "episode", "name": "Podcast Ep", "uri": "spotify:episode:e"},
            },  # episode -> skipped
        ],
        "next": "https://api.spotify.com/v1/playlists/PL123/tracks?offset=100&limit=100",
    }


@pytest.fixture
def tracks_page_2():
    return {
        "items": [
            _track_item("Sóng C", "Artíst C", "Album C", "spotify:track:c"),
        ],
        "next": None,
    }


@pytest.fixture
def search_response():
    return {
        "tracks": {
            "items": [
                {"uri": "spotify:track:found", "name": "Found", "artists": [{"name": "X"}]}
            ]
        }
    }


@pytest.fixture
def user_profile():
    return {"id": "alice", "display_name": "Alice"}
