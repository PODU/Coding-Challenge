"""CLI tests: arg parsing, flow selection, exit codes, offline backup/restore."""

from __future__ import annotations

import json

import pytest
import responses

from spotify_playlist import cli
from spotify_playlist.auth import TOKEN_URL


# --------------------------------------------------------------------------- #
# Argument parsing
# --------------------------------------------------------------------------- #
def test_parser_backup_defaults():
    args = cli.build_parser().parse_args(["backup", "PL123"])
    assert args.command == "backup"
    assert args.playlist_id == "PL123"
    assert args.zip is False
    assert args.user is False  # default = client-credentials


def test_parser_backup_user_and_zip():
    args = cli.build_parser().parse_args(["backup", "PL123", "--user", "--zip", "-o", "x.csv"])
    assert args.user is True
    assert args.zip is True
    assert args.output == "x.csv"


def test_parser_restore():
    args = cli.build_parser().parse_args(["restore", "in.csv", "--name", "My PL", "--public"])
    assert args.command == "restore"
    assert args.input == "in.csv"
    assert args.name == "My PL"
    assert args.public is True
    assert args.user is True  # restore always needs user auth


def test_parser_list_uses_user_flow():
    args = cli.build_parser().parse_args(["list"])
    assert args.command == "list"
    assert args.user is True


def test_restore_requires_name():
    with pytest.raises(SystemExit):
        cli.build_parser().parse_args(["restore", "in.csv"])


def test_playlist_id_parsing():
    assert cli._playlist_id("PL123") == "PL123"
    assert cli._playlist_id("spotify:playlist:ABC") == "ABC"
    assert cli._playlist_id("https://open.spotify.com/playlist/XYZ?si=1") == "XYZ"


# --------------------------------------------------------------------------- #
# Config / exit codes
# --------------------------------------------------------------------------- #
def test_missing_credentials_exit_code(monkeypatch, capsys):
    monkeypatch.delenv("SPOTIFY_CLIENT_ID", raising=False)
    monkeypatch.delenv("SPOTIFY_CLIENT_SECRET", raising=False)
    rc = cli.main(["backup", "PL123"])
    assert rc == 2
    assert "Configuration error" in capsys.readouterr().err


# --------------------------------------------------------------------------- #
# End-to-end backup (client-credentials, fully mocked)
# --------------------------------------------------------------------------- #
@responses.activate
def test_cli_backup_writes_csv(monkeypatch, tmp_path, dummy_env, playlist_meta, tracks_page_2):
    responses.post(TOKEN_URL, json={"access_token": "T", "expires_in": 3600})
    responses.get("https://api.spotify.com/v1/playlists/PL123", json=playlist_meta)
    responses.get("https://api.spotify.com/v1/playlists/PL123/tracks", json=tracks_page_2)

    out = tmp_path / "backup.csv"
    rc = cli.main(["backup", "PL123", "-o", str(out)])
    assert rc == 0
    assert out.is_file()
    assert "Sóng C" in out.read_text(encoding="utf-8")


@responses.activate
def test_cli_backup_zip(monkeypatch, tmp_path, dummy_env, playlist_meta, tracks_page_2):
    responses.post(TOKEN_URL, json={"access_token": "T", "expires_in": 3600})
    responses.get("https://api.spotify.com/v1/playlists/PL123", json=playlist_meta)
    responses.get("https://api.spotify.com/v1/playlists/PL123/tracks", json=tracks_page_2)

    out = tmp_path / "backup.zip"
    rc = cli.main(["backup", "PL123", "-o", str(out), "--zip"])
    assert rc == 0
    import zipfile

    assert zipfile.is_zipfile(out)


# --------------------------------------------------------------------------- #
# End-to-end restore (user flow via pre-seeded refresh token)
# --------------------------------------------------------------------------- #
@responses.activate
def test_cli_restore(monkeypatch, tmp_path, dummy_env):
    # Seed a token cache so the user flow refreshes instead of prompting.
    cache = tmp_path / "token.json"
    cache.write_text(json.dumps({"refresh_token": "R", "expires_at": 0}))
    monkeypatch.setenv("SPOTIFY_TOKEN_CACHE", str(cache))

    # Input CSV with one track.
    csv_path = tmp_path / "in.csv"
    csv_path.write_text(
        "track_name,artists,album,track_uri,duration_ms,added_at,isrc\n"
        "Song A,Artist A,Album A,spotify:track:a,200000,,US1\n",
        encoding="utf-8",
    )

    responses.post(TOKEN_URL, json={"access_token": "UT", "expires_in": 3600})
    responses.get("https://api.spotify.com/v1/me", json={"id": "alice"})
    responses.get(
        "https://api.spotify.com/v1/search",
        json={"tracks": {"items": [{"uri": "spotify:track:found"}]}},
    )
    responses.post(
        "https://api.spotify.com/v1/users/alice/playlists",
        json={"id": "NEWPL", "external_urls": {"spotify": "http://x/NEWPL"}},
    )
    added = []
    responses.post(
        "https://api.spotify.com/v1/playlists/NEWPL/tracks",
        json={"snapshot_id": "s"},
    )

    rc = cli.main(["restore", str(csv_path), "--name", "Restored"])
    assert rc == 0
    # Verify an add request was sent with the found uri.
    add_reqs = [c for c in responses.calls if c.request.url.endswith("/NEWPL/tracks")]
    assert len(add_reqs) == 1
    body = add_reqs[0].request.body
    if isinstance(body, bytes):
        body = body.decode("utf-8")
    assert "spotify:track:found" in body
