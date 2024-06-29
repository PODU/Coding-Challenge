"""ZIP header checks and encryption-scheme detection.

A ZIP's metadata (names, sizes, CRCs) stays readable even when the file
contents are encrypted, so the central directory can be read and the cheapest
entry chosen as the crack target without the password.
"""

from __future__ import annotations

import zipfile
from dataclasses import dataclass
from typing import Literal

_LOCAL_FILE_HEADER = b"PK\x03\x04"
_EOCD = b"PK\x05\x06"
_SPANNED = b"PK\x07\x08"

_FLAG_ENCRYPTED = 0x1  # general-purpose flag bit 0: entry is encrypted

# WinZip AES: compression method 99 and an extra field with header id 0x9901.
_AES_COMPRESSION = 99
_AES_EXTRA_ID = 0x9901

Scheme = Literal["zipcrypto", "aes", "none"]


@dataclass(frozen=True)
class EntryInfo:
    """An encrypted entry picked as the crack target."""

    name: str
    compress_size: int
    scheme: Scheme


def is_zip(path: str) -> bool:
    """Return ``True`` if *path* starts with a ZIP signature (a normal entry,
    an empty archive, or a spanned marker). Unreadable paths give ``False``."""
    try:
        with open(path, "rb") as fh:
            magic = fh.read(4)
    except OSError:
        return False
    return magic in (_LOCAL_FILE_HEADER, _EOCD, _SPANNED)


def _entry_is_aes(info: zipfile.ZipInfo) -> bool:
    if info.compress_type == _AES_COMPRESSION:
        return True
    # Scan the extra field for the AES header id.
    extra = info.extra
    i = 0
    while i + 4 <= len(extra):
        header_id = int.from_bytes(extra[i : i + 2], "little")
        data_size = int.from_bytes(extra[i + 2 : i + 4], "little")
        if header_id == _AES_EXTRA_ID:
            return True
        i += 4 + data_size
    return False


def _entry_is_encrypted(info: zipfile.ZipInfo) -> bool:
    return bool(info.flag_bits & _FLAG_ENCRYPTED)


def detect_scheme(path: str) -> Scheme:
    """Report the archive's encryption: ``"aes"`` if any entry uses WinZip AES,
    ``"zipcrypto"`` if any uses the legacy cipher, else ``"none"``."""
    with zipfile.ZipFile(path) as zf:
        saw_zipcrypto = False
        for info in zf.infolist():
            if not _entry_is_encrypted(info):
                continue
            if _entry_is_aes(info):
                return "aes"
            saw_zipcrypto = True
    return "zipcrypto" if saw_zipcrypto else "none"


def smallest_encrypted_entry(path: str) -> EntryInfo:
    """Return the encrypted entry with the smallest compressed size, which is
    the cheapest to decrypt per candidate. Raises :class:`ValueError` if there
    are no encrypted entries."""
    best: zipfile.ZipInfo | None = None
    best_is_aes = False
    with zipfile.ZipFile(path) as zf:
        for info in zf.infolist():
            if info.is_dir() or not _entry_is_encrypted(info):
                continue
            if best is None or info.compress_size < best.compress_size:
                best = info
                best_is_aes = _entry_is_aes(info)
    if best is None:
        raise ValueError(f"{path!r} has no encrypted entries")
    return EntryInfo(
        name=best.filename,
        compress_size=best.compress_size,
        scheme="aes" if best_is_aes else "zipcrypto",
    )
