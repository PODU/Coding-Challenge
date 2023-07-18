"""Tests for the SpotifyClient endpoint wrappers (all HTTP mocked)."""

from __future__ import annotations

import pytest
import responses

from spotify_client.config import API_BASE, TOKEN_URL
from spotify_client.errors import (
    ApiError,
    NoActiveDeviceError,
    PremiumRequiredError,
    RateLimited,
)


@responses.activate
def test_current_user_returns_profile(client):
    responses.add(responses.GET, f"{API_BASE}/me",
                  json={"id": "alice", "display_name": "Alice"}, status=200)
    profile = client.current_user()
    assert profile["id"] == "alice"


@responses.activate
def test_playlists_maps_to_dataclass(client):
    responses.add(
        responses.GET, f"{API_BASE}/me/playlists",
        json={
            "items": [
                {
                    "id": "p1", "name": "Chill", "uri": "spotify:playlist:p1",
                    "owner": {"display_name": "Alice"},
                    "tracks": {"total": 12},
                    "images": [{"url": "http://img/p1.jpg"}],
                }
            ],
            "next": None,
        },
        status=200,
    )
    pls = client.playlists()
    assert len(pls) == 1
    assert pls[0].name == "Chill"
    assert pls[0].track_count == 12
    assert pls[0].image_url == "http://img/p1.jpg"
    assert pls[0].owner == "Alice"


@responses.activate
def test_playlists_pagination_follows_next(client):
    page2 = f"{API_BASE}/me/playlists?offset=1"
    responses.add(
        responses.GET, f"{API_BASE}/me/playlists",
        json={"items": [{"id": "a", "name": "A"}], "next": page2},
        status=200,
    )
    responses.add(
        responses.GET, page2,
        json={"items": [{"id": "b", "name": "B"}], "next": None},
        status=200,
    )
    pls = client.playlists(limit=10)
    assert [p.name for p in pls] == ["A", "B"]


@responses.activate
def test_search_tracks_maps_nested_container(client):
    responses.add(
        responses.GET, f"{API_BASE}/search",
        json={
            "tracks": {
                "items": [
                    {
                        "id": "t1", "name": "Song", "uri": "spotify:track:t1",
                        "duration_ms": 1000,
                        "artists": [{"name": "Band"}],
                        "album": {"name": "Album"},
                    }
                ],
                "next": None,
            }
        },
        status=200,
    )
    tracks = client.search_tracks("song", limit=5)
    assert len(tracks) == 1
    assert tracks[0].name == "Song"
    assert tracks[0].artist_names == "Band"
    assert tracks[0].album == "Album"


@responses.activate
def test_devices_lists_devices(client):
    responses.add(
        responses.GET, f"{API_BASE}/me/player/devices",
        json={"devices": [
            {"id": "d1", "name": "Phone", "type": "Smartphone",
             "is_active": True, "volume_percent": 50}
        ]},
        status=200,
    )
    devs = client.devices()
    assert devs[0].name == "Phone"
    assert devs[0].is_active is True


@responses.activate
def test_current_playback_204_raises_no_active_device(client):
    responses.add(responses.GET, f"{API_BASE}/me/player", status=204)
    with pytest.raises(NoActiveDeviceError):
        client.current_playback()


@responses.activate
def test_current_playback_returns_state(client):
    responses.add(
        responses.GET, f"{API_BASE}/me/player",
        json={
            "is_playing": True,
            "progress_ms": 5000,
            "device": {"id": "d1", "name": "Desk", "is_active": True},
            "item": {"id": "t1", "name": "Song",
                     "artists": [{"name": "Band"}],
                     "album": {"name": "Album"}, "duration_ms": 1000},
        },
        status=200,
    )
    pb = client.current_playback()
    assert pb.is_playing
    assert pb.device.name == "Desk"
    assert pb.track.name == "Song"


@responses.activate
def test_premium_required_403_maps_to_typed_error(client):
    responses.add(
        responses.PUT, f"{API_BASE}/me/player/play",
        json={"error": {"status": 403, "reason": "PREMIUM_REQUIRED",
                        "message": "Player command failed: Premium required"}},
        status=403,
    )
    with pytest.raises(PremiumRequiredError):
        client.play()


@responses.activate
def test_404_device_maps_to_no_active_device(client):
    responses.add(
        responses.PUT, f"{API_BASE}/me/player/pause",
        json={"error": {"status": 404, "message": "Device not found"}},
        status=404,
    )
    with pytest.raises(NoActiveDeviceError):
        client.pause()


@responses.activate
def test_429_retries_then_succeeds(client):
    responses.add(responses.GET, f"{API_BASE}/me/player/devices",
                  status=429, headers={"Retry-After": "1"})
    responses.add(responses.GET, f"{API_BASE}/me/player/devices",
                  json={"devices": []}, status=200)
    devs = client.devices()
    assert devs == []
    assert len(responses.calls) == 2


@responses.activate
def test_429_exhausted_raises_rate_limited(client):
    responses.add(responses.GET, f"{API_BASE}/me/player/devices",
                  status=429, headers={"Retry-After": "1"})
    responses.add(responses.GET, f"{API_BASE}/me/player/devices",
                  status=429, headers={"Retry-After": "2"})
    with pytest.raises(RateLimited) as exc:
        client.devices()
    assert exc.value.retry_after == 2


@responses.activate
def test_401_triggers_single_refresh_and_retry(client, expired_token):
    # First call 401s; client should refresh then retry and succeed.
    responses.add(responses.GET, f"{API_BASE}/me", status=401,
                  json={"error": {"status": 401, "message": "expired"}})
    responses.add(responses.POST, TOKEN_URL,
                  json={"access_token": "fresh", "expires_in": 3600},
                  status=200)
    responses.add(responses.GET, f"{API_BASE}/me",
                  json={"id": "alice"}, status=200)
    profile = client.current_user()
    assert profile["id"] == "alice"
    # Exactly one token refresh occurred.
    token_calls = [c for c in responses.calls if c.request.url == TOKEN_URL]
    assert len(token_calls) == 1


@responses.activate
def test_transfer_sends_device_ids(client):
    responses.add(responses.PUT, f"{API_BASE}/me/player", status=204)
    client.transfer("dev-9")
    import json as jsonlib
    body = jsonlib.loads(responses.calls[0].request.body)
    assert body["device_ids"] == ["dev-9"]
    assert body["play"] is True


@responses.activate
def test_play_with_context_uri(client):
    responses.add(responses.PUT, f"{API_BASE}/me/player/play", status=204)
    client.play(context_uri="spotify:playlist:p1", device_id="d1")
    import json as jsonlib
    req = responses.calls[0].request
    assert "device_id=d1" in req.url
    body = jsonlib.loads(req.body)
    assert body["context_uri"] == "spotify:playlist:p1"


@responses.activate
def test_resolve_device_by_name(client):
    responses.add(
        responses.GET, f"{API_BASE}/me/player/devices",
        json={"devices": [
            {"id": "d1", "name": "Kitchen Speaker", "is_active": False}
        ]},
        status=200,
    )
    dev = client.resolve_device("kitchen speaker")
    assert dev.id == "d1"


@responses.activate
def test_resolve_device_not_found(client):
    responses.add(responses.GET, f"{API_BASE}/me/player/devices",
                  json={"devices": []}, status=200)
    with pytest.raises(ApiError):
        client.resolve_device("nope")
