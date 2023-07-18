"""Dataclasses for the bits of Spotify's JSON we use."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


def _first_image_url(images: Optional[List[Dict[str, Any]]]) -> Optional[str]:
    if not images:
        return None
    return images[0].get("url")


@dataclass
class Track:
    id: Optional[str]
    name: str
    artists: List[str] = field(default_factory=list)
    album: Optional[str] = None
    uri: Optional[str] = None
    duration_ms: int = 0

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Track":
        album = data.get("album") or {}
        return cls(
            id=data.get("id"),
            name=data.get("name", ""),
            artists=[a.get("name", "") for a in data.get("artists", [])],
            album=album.get("name"),
            uri=data.get("uri"),
            duration_ms=int(data.get("duration_ms") or 0),
        )

    @property
    def artist_names(self) -> str:
        return ", ".join(a for a in self.artists if a)


@dataclass
class Playlist:
    id: Optional[str]
    name: str
    uri: Optional[str] = None
    owner: Optional[str] = None
    track_count: int = 0
    image_url: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Playlist":
        owner = data.get("owner") or {}
        tracks = data.get("tracks") or {}
        return cls(
            id=data.get("id"),
            name=data.get("name", ""),
            uri=data.get("uri"),
            owner=owner.get("display_name") or owner.get("id"),
            track_count=int(tracks.get("total") or 0),
            image_url=_first_image_url(data.get("images")),
        )


@dataclass
class Device:
    id: Optional[str]
    name: str
    type: Optional[str] = None
    is_active: bool = False
    volume_percent: Optional[int] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Device":
        return cls(
            id=data.get("id"),
            name=data.get("name", ""),
            type=data.get("type"),
            is_active=bool(data.get("is_active")),
            volume_percent=data.get("volume_percent"),
        )


@dataclass
class Playback:
    """``GET /v1/me/player``."""

    is_playing: bool = False
    device: Optional[Device] = None
    track: Optional[Track] = None
    progress_ms: int = 0

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Playback":
        device = data.get("device")
        item = data.get("item")
        return cls(
            is_playing=bool(data.get("is_playing")),
            device=Device.from_dict(device) if device else None,
            track=Track.from_dict(item) if item else None,
            progress_ms=int(data.get("progress_ms") or 0),
        )


__all__ = ["Track", "Playlist", "Device", "Playback"]
