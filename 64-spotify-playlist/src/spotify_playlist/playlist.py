"""Backing up and restoring playlists.

A restore can't reuse the old track URIs blindly, so each track is looked up
again, by ISRC if we have one and by name and artist otherwise.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Optional

from .client import SpotifyClient
from .models import Playlist, Track

log = logging.getLogger(__name__)

# Spotify accepts at most 100 URIs per add request.
ADD_CHUNK_SIZE = 100


def track_from_item(item: Dict[str, Any]) -> Optional[Track]:
    """Convert one playlist item to a :class:`Track`.

    Returns ``None`` for things we can't back up: removed tracks, local files
    and podcast episodes.
    """
    if not isinstance(item, dict):
        return None
    track = item.get("track")
    if not track:
        log.warning("Skipping playlist item with no track data")
        return None
    if item.get("is_local") or track.get("is_local"):
        log.warning("Skipping local track: %s", track.get("name"))
        return None
    if track.get("type", "track") != "track":
        log.warning("Skipping non-track item (type=%s): %s", track.get("type"), track.get("name"))
        return None

    artists = [a.get("name", "") for a in track.get("artists", []) if a.get("name")]
    album = (track.get("album") or {}).get("name", "")
    isrc = (track.get("external_ids") or {}).get("isrc")

    return Track(
        name=track.get("name", ""),
        artists=artists,
        album=album,
        uri=track.get("uri", ""),
        duration_ms=track.get("duration_ms"),
        added_at=item.get("added_at"),
        isrc=isrc,
    )


def backup_playlist(client: SpotifyClient, playlist_id: str) -> Playlist:
    """Fetch a playlist with all of its tracks."""
    meta = client.get(f"/v1/playlists/{playlist_id}")
    owner_info = meta.get("owner") or {}
    owner = owner_info.get("display_name") or owner_info.get("id", "")

    tracks: List[Track] = []
    for item in client.get_paginated(
        f"/v1/playlists/{playlist_id}/tracks", params={"limit": ADD_CHUNK_SIZE}
    ):
        track = track_from_item(item)
        if track is not None:
            tracks.append(track)

    pl = Playlist(
        id=meta.get("id", playlist_id),
        name=meta.get("name", ""),
        owner=owner,
        description=meta.get("description", ""),
        tracks=tracks,
    )
    log.info("Backed up playlist %r (%d tracks)", pl.name, len(pl))
    return pl


@dataclass
class RestoreResult:
    playlist_id: str
    playlist_url: str = ""
    added: int = 0
    duplicates: int = 0
    misses: List[Dict[str, str]] = field(default_factory=list)


def _strip_quotes(value: str) -> str:
    # Spotify's search syntax has no way to escape a quote inside a field.
    return value.replace('"', "")


def _search_tracks(client: SpotifyClient, query: str) -> List[Dict[str, Any]]:
    result = client.get("/v1/search", params={"q": query, "type": "track", "limit": 1})
    return ((result.get("tracks") or {}).get("items")) or []


def search_track_uri(client: SpotifyClient, row: Dict[str, str]) -> Optional[str]:
    """Find the track for a backup row: by ISRC if there is one, otherwise by
    name and first artist. Returns ``None`` if nothing matches.
    """
    isrc = (row.get("isrc") or "").strip()
    if isrc:
        items = _search_tracks(client, f"isrc:{isrc}")
        if items:
            return items[0].get("uri")

    name = _strip_quotes((row.get("track_name") or "").strip())
    artists = (row.get("artists") or "").strip()
    first_artist = _strip_quotes(artists.split(";")[0].strip()) if artists else ""
    if not name:
        return None

    query = f'track:"{name}"'
    if first_artist:
        query += f' artist:"{first_artist}"'
    items = _search_tracks(client, query)
    return items[0].get("uri") if items else None


def chunked(seq: List[Any], size: int) -> Iterable[List[Any]]:
    """Split ``seq`` into lists of at most ``size`` items."""
    for i in range(0, len(seq), size):
        yield seq[i : i + size]


def current_user_id(client: SpotifyClient) -> str:
    return client.get("/v1/me").get("id", "")


def restore_playlist(
    client: SpotifyClient,
    rows: Iterable[Dict[str, str]],
    *,
    name: str,
    description: str = "Restored by spotify-playlist",
    public: bool = False,
    user_id: Optional[str] = None,
) -> RestoreResult:
    """Create a new playlist from backup rows.

    Tracks that can't be found are listed in the result rather than failing
    the restore, and tracks that appear twice are only added once.
    """
    if user_id is None:
        user_id = current_user_id(client)

    uris: List[str] = []
    seen: set[str] = set()
    duplicates = 0
    misses: List[Dict[str, str]] = []

    for row in rows:
        uri = search_track_uri(client, row)
        if not uri:
            log.warning("No match for %r by %r", row.get("track_name"), row.get("artists"))
            misses.append(dict(row))
            continue
        if uri in seen:
            duplicates += 1
            continue
        seen.add(uri)
        uris.append(uri)

    created = client.post(
        f"/v1/users/{user_id}/playlists",
        json={"name": name, "description": description, "public": public},
    )
    playlist_id = created.get("id", "")
    playlist_url = (created.get("external_urls") or {}).get("spotify", "")

    for chunk in chunked(uris, ADD_CHUNK_SIZE):
        client.post(f"/v1/playlists/{playlist_id}/tracks", json={"uris": chunk})

    log.info(
        "Restored %d tracks into playlist %s (%d duplicates skipped, %d misses)",
        len(uris),
        playlist_id,
        duplicates,
        len(misses),
    )
    return RestoreResult(
        playlist_id=playlist_id,
        playlist_url=playlist_url,
        added=len(uris),
        duplicates=duplicates,
        misses=misses,
    )


def list_my_playlists(client: SpotifyClient) -> List[Dict[str, Any]]:
    """Return all of the current user's playlists."""
    return list(client.get_paginated("/v1/me/playlists", params={"limit": 50}))
