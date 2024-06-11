"""Exporter tests: CSV header/quoting, zip, round-trip."""

from __future__ import annotations

import csv
import zipfile

import pytest

from spotify_playlist.exporter import CSV_HEADER, read_csv, write_csv, write_zip
from spotify_playlist.models import Playlist, Track


@pytest.fixture
def sample_playlist():
    return Playlist(
        id="PL123",
        name="My Mix",
        owner="Alice",
        tracks=[
            Track(
                name="Song A",
                artists=["Artist A", "Feat B"],
                album="Album A",
                uri="spotify:track:a",
                duration_ms=200000,
                added_at="2021-01-01T00:00:00Z",
                isrc="US0000000001",
            ),
            Track(
                name='Song B, with "comma"',
                artists=["Artíst Ünïcode"],
                album="Album B",
                uri="spotify:track:b",
                duration_ms=None,
            ),
        ],
    )


def test_write_csv_header_and_rows(sample_playlist, tmp_path):
    path = write_csv(sample_playlist, tmp_path / "out.csv")
    with path.open(encoding="utf-8", newline="") as fh:
        rows = list(csv.reader(fh))
    assert rows[0] == CSV_HEADER
    assert rows[1][0] == "Song A"
    assert rows[1][1] == "Artist A; Feat B"  # joined with "; "
    assert rows[1][6] == "US0000000001"
    assert rows[2][4] == ""  # None duration -> empty


def test_csv_quoting_of_commas_quotes_unicode(sample_playlist, tmp_path):
    path = write_csv(sample_playlist, tmp_path / "out.csv")
    text = path.read_text(encoding="utf-8")
    # Field with comma+quote must be quoted/escaped by the csv module.
    assert '"Song B, with ""comma"""' in text
    assert "Artíst Ünïcode" in text


def test_roundtrip_csv(sample_playlist, tmp_path):
    path = write_csv(sample_playlist, tmp_path / "out.csv")
    rows = read_csv(path)
    assert len(rows) == 2
    assert rows[0]["track_name"] == "Song A"
    assert rows[0]["artists"] == "Artist A; Feat B"
    assert rows[0]["isrc"] == "US0000000001"
    assert rows[1]["track_name"] == 'Song B, with "comma"'
    assert rows[1]["artists"] == "Artíst Ünïcode"


def test_write_zip_contains_csv(sample_playlist, tmp_path):
    path = write_zip(sample_playlist, tmp_path / "out.zip")
    assert zipfile.is_zipfile(path)
    with zipfile.ZipFile(path) as zf:
        names = zf.namelist()
    assert any(n.endswith(".csv") for n in names)


def test_roundtrip_zip(sample_playlist, tmp_path):
    path = write_zip(sample_playlist, tmp_path / "out.zip")
    rows = read_csv(path)
    assert len(rows) == 2
    assert rows[0]["track_name"] == "Song A"
