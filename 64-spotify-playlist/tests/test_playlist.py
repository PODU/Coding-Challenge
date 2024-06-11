"""Playlist tests: backup normalization, restore search/dedupe/chunking."""

from __future__ import annotations

import json

import pytest
import responses

from spotify_playlist.client import SpotifyClient
from spotify_playlist.playlist import (
    ADD_CHUNK_SIZE,
    backup_playlist,
    chunked,
    restore_playlist,
    search_track_uri,
    track_from_item,
)


class StaticProvider:
    def get_token(self) -> str:
        return "TOKEN"

    def refresh(self) -> str:
        return "TOKEN"


@pytest.fixture
def client():
    return SpotifyClient(StaticProvider())


def test_track_from_item_normalizes(tracks_page_1):
    track = track_from_item(tracks_page_1["items"][0])
    assert track is not None
    assert track.name == "Song A"
    assert track.artists == ["Artist A"]
    assert track.album == "Album A"
    assert track.uri == "spotify:track:a"
    assert track.isrc == "US0000000001"


def test_track_from_item_skips_null_local_episode(tracks_page_1):
    # index 2 = null track, index 3 = episode
    assert track_from_item(tracks_page_1["items"][2]) is None
    assert track_from_item(tracks_page_1["items"][3]) is None
    local = {"is_local": True, "track": {"type": "track", "name": "L", "uri": "x"}}
    assert track_from_item(local) is None


@responses.activate
def test_backup_playlist_paginates_and_filters(
    client, playlist_meta, tracks_page_1, tracks_page_2
):
    responses.get("https://api.spotify.com/v1/playlists/PL123", json=playlist_meta)
    responses.get(
        "https://api.spotify.com/v1/playlists/PL123/tracks",
        json=tracks_page_1,
        match=[responses.matchers.query_param_matcher({"limit": "100"})],
    )
    responses.get(
        "https://api.spotify.com/v1/playlists/PL123/tracks",
        json=tracks_page_2,
        match=[responses.matchers.query_param_matcher({"offset": "100", "limit": "100"})],
    )
    pl = backup_playlist(client, "PL123")
    assert pl.name == "My Mix"
    assert pl.owner == "Alice"
    # 2 valid on page 1 (+null+episode skipped) + 1 on page 2 = 3 tracks
    assert len(pl) == 3
    assert [t.name for t in pl.tracks] == ["Song A", "Song B, with comma", "Sóng C"]


@responses.activate
def test_search_prefers_isrc(client):
    responses.get(
        "https://api.spotify.com/v1/search",
        json={"tracks": {"items": [{"uri": "spotify:track:byisrc"}]}},
        match=[responses.matchers.query_param_matcher(
            {"q": "isrc:US123", "type": "track", "limit": "1"}
        )],
    )
    uri = search_track_uri(client, {"isrc": "US123", "track_name": "x", "artists": "y"})
    assert uri == "spotify:track:byisrc"


@responses.activate
def test_search_falls_back_to_name_artist(client):
    responses.get(
        "https://api.spotify.com/v1/search",
        json={"tracks": {"items": [{"uri": "spotify:track:byname"}]}},
        match=[responses.matchers.query_param_matcher(
            {"q": 'track:"Song A" artist:"Artist A"', "type": "track", "limit": "1"}
        )],
    )
    uri = search_track_uri(client, {"track_name": "Song A", "artists": "Artist A; Other"})
    assert uri == "spotify:track:byname"


@responses.activate
def test_search_miss_returns_none(client):
    responses.get("https://api.spotify.com/v1/search", json={"tracks": {"items": []}})
    assert search_track_uri(client, {"track_name": "Nope", "artists": "Nobody"}) is None


def test_chunked():
    assert list(chunked(list(range(5)), 2)) == [[0, 1], [2, 3], [4]]
    assert list(chunked([], 2)) == []


@responses.activate
def test_restore_dedupes_and_chunks_at_100(client):
    # 250 rows, but search always returns the SAME uri for 'dup', plus uniques.
    # Build rows: 150 unique + 100 duplicates of the first unique.
    rows = [{"track_name": f"T{i}", "artists": "A"} for i in range(150)]
    rows += [{"track_name": "T0", "artists": "A"} for _ in range(100)]

    def search_callback(request):
        from urllib.parse import parse_qs, urlparse

        q = parse_qs(urlparse(request.url).query)["q"][0]
        # extract Tn name
        name = q.split('"')[1]
        return (200, {}, json.dumps({"tracks": {"items": [{"uri": f"spotify:track:{name}"}]}}))

    responses.add_callback(
        responses.GET,
        "https://api.spotify.com/v1/search",
        callback=search_callback,
        content_type="application/json",
    )
    responses.get("https://api.spotify.com/v1/me", json={"id": "alice"})
    responses.post(
        "https://api.spotify.com/v1/users/alice/playlists",
        json={"id": "NEWPL", "external_urls": {"spotify": "http://x/NEWPL"}},
    )

    add_calls = []

    def add_callback(request):
        body = json.loads(request.body)
        add_calls.append(len(body["uris"]))
        return (201, {}, json.dumps({"snapshot_id": "s"}))

    responses.add_callback(
        responses.POST,
        "https://api.spotify.com/v1/playlists/NEWPL/tracks",
        callback=add_callback,
        content_type="application/json",
    )

    result = restore_playlist(client, rows, name="Restored")
    assert result.playlist_id == "NEWPL"
    assert result.added == 150  # 100 dup-of-T0 collapsed
    assert result.duplicates == 100
    # 150 URIs -> chunks of 100 => [100, 50]
    assert add_calls == [ADD_CHUNK_SIZE, 50]


@responses.activate
def test_restore_records_misses(client):
    responses.get("https://api.spotify.com/v1/search", json={"tracks": {"items": []}})
    responses.get("https://api.spotify.com/v1/me", json={"id": "alice"})
    responses.post(
        "https://api.spotify.com/v1/users/alice/playlists", json={"id": "NEWPL"}
    )
    rows = [{"track_name": "Ghost", "artists": "Nobody"}]
    result = restore_playlist(client, rows, name="R")
    assert result.added == 0
    assert len(result.misses) == 1
    assert result.misses[0]["track_name"] == "Ghost"
