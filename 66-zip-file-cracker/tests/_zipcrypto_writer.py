"""Build a genuine legacy-ZipCrypto-encrypted ZIP, in pure Python.

Neither the stdlib ``zipfile`` nor ``pyzipper`` can *write* the traditional
PKWARE stream cipher, so the test suite needs its own minimal writer to produce
a deterministic, stdlib-readable ZipCrypto fixture. The entry is STORED (no
compression) to keep the maths obvious; the produced archive is exactly what
``zipfile.ZipFile(...).read(name, pwd=...)`` expects.

This is intentionally small and self-contained. It is test-support code, not
part of the shipped package.
"""

from __future__ import annotations

import os
import struct
import zlib

# CRC-32 table (reflected, polynomial 0xEDB88320) - the same one the PKWARE
# cipher and zipfile's decrypter use.
_CRC_TABLE: list[int] = []
for _i in range(256):
    _c = _i
    for _ in range(8):
        _c = (_c >> 1) ^ 0xEDB88320 if _c & 1 else _c >> 1
    _CRC_TABLE.append(_c)


def _crc32_byte(crc: int, ch: int) -> int:
    return (crc >> 8) ^ _CRC_TABLE[(crc ^ ch) & 0xFF]


class _Cipher:
    """Traditional PKWARE stream cipher (encryption direction)."""

    def __init__(self, password: bytes) -> None:
        self.k0 = 0x12345678
        self.k1 = 0x23456789
        self.k2 = 0x34567890
        for b in password:
            self._update(b)

    def _update(self, ch: int) -> None:
        self.k0 = _crc32_byte(self.k0, ch)
        self.k1 = (self.k1 + (self.k0 & 0xFF)) & 0xFFFFFFFF
        self.k1 = (self.k1 * 134775813 + 1) & 0xFFFFFFFF
        self.k2 = _crc32_byte(self.k2, (self.k1 >> 24) & 0xFF)

    def _keystream_byte(self) -> int:
        temp = (self.k2 | 2) & 0xFFFF
        return ((temp * (temp ^ 1)) >> 8) & 0xFF

    def encrypt(self, data: bytes) -> bytes:
        out = bytearray(len(data))
        for i, plain in enumerate(data):
            out[i] = plain ^ self._keystream_byte()
            # Encryption updates the keys with the *plaintext* byte.
            self._update(plain)
        return bytes(out)


def build_zipcrypto_zip(
    filename: str,
    data: bytes,
    password: str,
    *,
    seed: int = 0,
) -> bytes:
    """Return the bytes of a single-entry ZipCrypto archive (STORED entry).

    *seed* makes the 12-byte encryption header deterministic when non-zero
    (handy for reproducible fixtures); otherwise random bytes are used.
    """
    pwd = password.encode("utf-8")
    crc = zlib.crc32(data) & 0xFFFFFFFF

    # 12-byte encryption header: 11 (pseudo)random bytes + a check byte equal to
    # the high byte of the CRC-32 (because general-purpose bit 3 is NOT set).
    if seed:
        rng = bytes(((seed * (i + 7)) & 0xFF) for i in range(11))
    else:
        rng = os.urandom(11)
    header = rng + bytes([(crc >> 24) & 0xFF])

    cipher = _Cipher(pwd)
    enc = cipher.encrypt(header + data)

    name = filename.encode("utf-8")
    flag = 0x0001  # bit 0: encrypted
    method = 0  # STORED
    mod_time = 0
    mod_date = 0x21  # 1980-01-01, valid DOS date
    comp_size = len(enc)
    uncomp_size = len(data)

    local = struct.pack(
        "<4s5H3L2H",
        b"PK\x03\x04",
        20,            # version needed
        flag,
        method,
        mod_time,
        mod_date,
        crc,
        comp_size,
        uncomp_size,
        len(name),
        0,             # extra length
    ) + name
    local_offset = 0
    local_record = local + enc

    central = struct.pack(
        "<4s6H3L5H2L",
        b"PK\x01\x02",
        20,            # version made by
        20,            # version needed
        flag,
        method,
        mod_time,
        mod_date,
        crc,
        comp_size,
        uncomp_size,
        len(name),
        0,             # extra length
        0,             # comment length
        0,             # disk number start
        0,             # internal attrs
        0,             # external attrs
        local_offset,
    ) + name

    eocd = struct.pack(
        "<4s4H2LH",
        b"PK\x05\x06",
        0,             # disk number
        0,             # disk with central dir
        1,             # entries this disk
        1,             # total entries
        len(central),
        len(local_record),
        0,             # comment length
    )
    return local_record + central + eocd
