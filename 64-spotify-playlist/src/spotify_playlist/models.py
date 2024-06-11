"""Playlist and track data."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class Track:
    name: str
    artists: List[str] = field(default_factory=list)
    album: str = ""
    uri: str = ""
    duration_ms: Optional[int] = None
    added_at: Optional[str] = None
    isrc: Optional[str] = None

    @property
    def artists_str(self) -> str:
        """The artists joined with ``"; "``, as stored in the CSV."""
        return "; ".join(self.artists)


@dataclass
class Playlist:
    id: str
    name: str
    owner: str = ""
    description: str = ""
    tracks: List[Track] = field(default_factory=list)

    def __len__(self) -> int:
        return len(self.tracks)
