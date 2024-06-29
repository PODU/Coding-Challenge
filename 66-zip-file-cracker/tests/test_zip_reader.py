"""Header validation and scheme detection (challenge Step 1 + scheme picks)."""

from __future__ import annotations

from pathlib import Path

from cczipcrack import zip_reader
from conftest import requires_pyzipper


def test_is_zip_true_for_zipcrypto(zipcrypto_zip: Path) -> None:
    assert zip_reader.is_zip(str(zipcrypto_zip)) is True


def test_is_zip_false_for_plain_file(non_zip: Path) -> None:
    assert zip_reader.is_zip(str(non_zip)) is False


def test_is_zip_false_for_missing_file(tmp_path: Path) -> None:
    assert zip_reader.is_zip(str(tmp_path / "nope.zip")) is False


def test_detect_scheme_zipcrypto(zipcrypto_zip: Path) -> None:
    assert zip_reader.detect_scheme(str(zipcrypto_zip)) == "zipcrypto"


def test_detect_scheme_none_for_plain(plain_zip: Path) -> None:
    assert zip_reader.detect_scheme(str(plain_zip)) == "none"


@requires_pyzipper
def test_detect_scheme_aes(aes_zip: Path) -> None:
    assert zip_reader.detect_scheme(str(aes_zip)) == "aes"


def test_smallest_encrypted_entry(zipcrypto_zip: Path) -> None:
    entry = zip_reader.smallest_encrypted_entry(str(zipcrypto_zip))
    assert entry.name == "cc.txt"
    assert entry.scheme == "zipcrypto"
