"""Tests for CLI dispatch with the SpotifyClient mocked at the boundary."""

from __future__ import annotations

import json as jsonlib
from unittest.mock import MagicMock

import pytest

from spotify_client import cli
from spotify_client.errors import AuthError, NoActiveDeviceError
from spotify_client.models import Device, Playback, Playlist, Track


@pytest.fixture
def fake_client(monkeypatch):
    """Patch ``build_client`` so the CLI uses a MagicMock client."""
    client = MagicMock()
    monkeypatch.setattr(cli, "build_client", lambda config=None: client)
    return client


def run(argv):
    """Invoke the CLI and return the exit code."""
    return cli.main(argv)


def test_me_calls_current_user(fake_client, capsys):
    fake_client.current_user.return_value = {"id": "alice", "display_name": "Alice"}
    code = run(["me"])
    assert code == 0
    fake_client.current_user.assert_called_once()
    assert "Alice" in capsys.readouterr().out


def test_playlists_dispatch_and_limit(fake_client, capsys):
    fake_client.playlists.return_value = [
        Playlist(id="p1", name="Chill", uri="spotify:playlist:p1",
                 owner="Alice", track_count=5, image_url="http://img")
    ]
    code = run(["playlists", "--limit", "7"])
    assert code == 0
    fake_client.playlists.assert_called_once_with(limit=7)
    out = capsys.readouterr().out
    assert "Chill" in out
    assert "http://img" in out


def test_search_passes_query(fake_client, capsys):
    fake_client.search_tracks.return_value = [
        Track(id="t1", name="Song", artists=["Band"], album="Album",
              uri="spotify:track:t1")
    ]
    code = run(["search", "hello", "--limit", "3"])
    assert code == 0
    fake_client.search_tracks.assert_called_once_with("hello", limit=3)
    assert "Song" in capsys.readouterr().out


def test_devices_json_output(fake_client, capsys):
    fake_client.devices.return_value = [
        Device(id="d1", name="Phone", type="Smartphone", is_active=True,
               volume_percent=50)
    ]
    code = run(["--json", "devices"])
    assert code == 0
    data = jsonlib.loads(capsys.readouterr().out)
    assert data[0]["name"] == "Phone"
    assert data[0]["is_active"] is True


def test_now_dispatch(fake_client, capsys):
    fake_client.current_playback.return_value = Playback(
        is_playing=True,
        device=Device(id="d1", name="Desk"),
        track=Track(id="t1", name="Song", artists=["Band"], album="Album"),
    )
    assert run(["now"]) == 0
    out = capsys.readouterr().out
    assert "Song" in out and "Desk" in out


def test_pause_dispatch(fake_client, capsys):
    fake_client.resolve_device.return_value = Device(id="d1", name="Desk")
    assert run(["pause", "--device", "Desk"]) == 0
    fake_client.resolve_device.assert_called_once_with("Desk")
    fake_client.pause.assert_called_once_with(device_id="d1")


def test_next_and_prev(fake_client):
    assert run(["next"]) == 0
    fake_client.next.assert_called_once_with(device_id=None)
    assert run(["prev"]) == 0
    fake_client.previous.assert_called_once_with(device_id=None)


def test_transfer_resolves_device(fake_client, capsys):
    fake_client.resolve_device.return_value = Device(id="dev-9", name="TV")
    assert run(["transfer", "TV"]) == 0
    fake_client.resolve_device.assert_called_once_with("TV")
    fake_client.transfer.assert_called_once_with("dev-9", play=True)
    assert "TV" in capsys.readouterr().out


def test_play_with_playlist_uri(fake_client, capsys):
    assert run(["play", "--playlist", "spotify:playlist:p1"]) == 0
    fake_client.play.assert_called_once_with(
        context_uri="spotify:playlist:p1", device_id=None)


def test_play_with_playlist_index(fake_client):
    fake_client.playlists.return_value = [
        Playlist(id="p0", name="Zero", uri="spotify:playlist:p0"),
        Playlist(id="p1", name="One", uri="spotify:playlist:p1"),
    ]
    assert run(["play", "--playlist", "1"]) == 0
    fake_client.play.assert_called_once_with(
        context_uri="spotify:playlist:p1", device_id=None)


def test_no_active_device_exit_code(fake_client, capsys):
    fake_client.current_playback.side_effect = NoActiveDeviceError("No active device.")
    code = run(["now"])
    assert code == cli.EXIT_NO_DEVICE
    assert "No active device" in capsys.readouterr().err


def test_auth_error_exit_code(fake_client, capsys):
    fake_client.current_user.side_effect = AuthError("login again")
    code = run(["me"])
    assert code == cli.EXIT_AUTH
    assert "Authentication error" in capsys.readouterr().err


def test_config_error_exit_code(monkeypatch, capsys):
    from spotify_client.errors import ConfigError

    def boom(config=None):
        raise ConfigError("SPOTIFY_CLIENT_ID is not set.")

    monkeypatch.setattr(cli, "build_client", boom)
    code = run(["me"])
    assert code == cli.EXIT_ERROR
    assert "Configuration error" in capsys.readouterr().err


def test_logout_clears_store(fake_client, capsys):
    assert run(["logout"]) == 0
    fake_client.auth.store.clear.assert_called_once()
