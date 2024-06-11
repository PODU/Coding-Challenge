"""Reading and writing backups as CSV, optionally zipped."""

from __future__ import annotations

import csv
import io
import logging
import os
import zipfile
from pathlib import Path
from typing import Dict, List

from .models import Playlist, Track

log = logging.getLogger(__name__)

CSV_HEADER = [
    "track_name",
    "artists",
    "album",
    "track_uri",
    "duration_ms",
    "added_at",
    "isrc",
]


def _row_for(track: Track) -> Dict[str, str]:
    return {
        "track_name": track.name,
        "artists": track.artists_str,
        "album": track.album,
        "track_uri": track.uri,
        "duration_ms": "" if track.duration_ms is None else str(track.duration_ms),
        "added_at": track.added_at or "",
        "isrc": track.isrc or "",
    }


def write_csv_to(fh, playlist: Playlist) -> None:
    """Write the playlist as CSV to an open text file."""
    writer = csv.DictWriter(fh, fieldnames=CSV_HEADER, lineterminator="\n")
    writer.writeheader()
    for track in playlist.tracks:
        writer.writerow(_row_for(track))


def write_csv(playlist: Playlist, path: str | os.PathLike[str]) -> Path:
    """Write the playlist to ``path`` as UTF-8 CSV and return the path."""
    p = Path(path)
    with p.open("w", encoding="utf-8", newline="") as fh:
        write_csv_to(fh, playlist)
    log.info("Wrote %d tracks to %s", len(playlist), p)
    return p


def write_zip(playlist: Playlist, path: str | os.PathLike[str]) -> Path:
    """Write the playlist as a CSV inside a zip file."""
    p = Path(path)
    buf = io.StringIO()
    write_csv_to(buf, playlist)

    arcname = (p.stem if p.suffix.lower() == ".zip" else p.name) + ".csv"
    with zipfile.ZipFile(p, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr(arcname, buf.getvalue())
    log.info("Wrote %d tracks to %s (%s)", len(playlist), p, arcname)
    return p


def read_csv(path: str | os.PathLike[str]) -> List[Dict[str, str]]:
    """Read a backup (CSV or zipped CSV) into a list of rows."""
    p = Path(path)
    if zipfile.is_zipfile(p):
        with zipfile.ZipFile(p) as zf:
            name = next((n for n in zf.namelist() if n.lower().endswith(".csv")), None)
            if name is None:
                raise ValueError(f"No CSV found inside zip: {p}")
            text = zf.read(name).decode("utf-8")
        return list(csv.DictReader(io.StringIO(text)))

    with p.open("r", encoding="utf-8", newline="") as fh:
        return list(csv.DictReader(fh))
