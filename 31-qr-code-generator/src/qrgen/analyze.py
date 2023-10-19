"""Choose the encoding mode and the smallest version that fits the input."""
from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from . import tables
from .tables import (
    MODE_ALPHANUMERIC,
    MODE_BYTE,
    MODE_KANJI,
    MODE_NUMERIC,
    ALPHANUMERIC_MAP,
)


class CapacityError(ValueError):
    """The payload doesn't fit at the requested EC level and version."""


class ModeError(ValueError):
    """The input can't be encoded in the requested mode."""


@dataclass(frozen=True)
class EncodingPlan:
    mode: str
    version: int
    ec: str
    text: str


def _is_numeric(text: str) -> bool:
    return len(text) > 0 and all(c in "0123456789" for c in text)


def _is_alphanumeric(text: str) -> bool:
    return len(text) > 0 and all(c in ALPHANUMERIC_MAP for c in text)


def _is_latin1(text: str) -> bool:
    try:
        text.encode("latin-1")
        return True
    except UnicodeEncodeError:
        return False


def _is_kanji(text: str) -> bool:
    """True if every character is a two-byte Shift-JIS code QR's Kanji mode covers."""
    if len(text) == 0:
        return False
    try:
        raw = text.encode("shift_jis")
    except UnicodeEncodeError:
        return False
    if len(raw) != 2 * len(text):
        return False
    for i in range(0, len(raw), 2):
        word = (raw[i] << 8) | raw[i + 1]
        if not (0x8140 <= word <= 0x9FFC or 0xE040 <= word <= 0xEBBF):
            return False
    return True


def detect_mode(text: str) -> str:
    """Return the most compact mode that can hold ``text``."""
    if _is_numeric(text):
        return MODE_NUMERIC
    if _is_alphanumeric(text):
        return MODE_ALPHANUMERIC
    if _is_latin1(text):
        return MODE_BYTE
    if _is_kanji(text):
        return MODE_KANJI
    # Anything else goes out as UTF-8 bytes.
    return MODE_BYTE


def validate_mode(text: str, mode: str) -> None:
    """Raise ModeError if ``text`` can't be encoded in ``mode``."""
    if mode == MODE_NUMERIC:
        for i, c in enumerate(text):
            if c not in "0123456789":
                raise ModeError(f"character {c!r} at index {i} is not numeric")
    elif mode == MODE_ALPHANUMERIC:
        for i, c in enumerate(text):
            if c not in ALPHANUMERIC_MAP:
                raise ModeError(
                    f"character {c!r} at index {i} is not alphanumeric"
                )
    elif mode == MODE_BYTE:
        pass  # anything can be sent as bytes
    elif mode == MODE_KANJI:
        if not _is_kanji(text):
            raise ModeError("input is not valid Shift-JIS Kanji")
    else:  # pragma: no cover - guarded by argparse choices
        raise ValueError(f"unknown mode: {mode}")


def _segment_bit_length(text: str, mode: str, version: int) -> int:
    """Bits for the mode indicator, character count and data, before padding."""
    from .encode import data_bit_length  # encode imports this module

    count_bits = tables.char_count_bits(mode, version)
    return 4 + count_bits + data_bit_length(text, mode)


def choose_version(
    text: str, mode: str, ec: str, forced_version: Optional[int] = None
) -> int:
    """Return the smallest version that fits ``text``, or check the forced one."""
    if forced_version is not None:
        need = _segment_bit_length(text, mode, forced_version)
        cap = tables.data_capacity_bits(forced_version, ec)
        if need > cap:
            raise CapacityError(
                f"payload needs {need} bits but version {forced_version}/{ec} "
                f"holds only {cap}"
            )
        return forced_version
    for version in range(1, 41):
        need = _segment_bit_length(text, mode, version)
        if need <= tables.data_capacity_bits(version, ec):
            return version
    biggest = tables.data_capacity_bits(40, ec)
    raise CapacityError(
        f"payload needs more than version 40/{ec} capacity ({biggest} bits)"
    )


def make_plan(
    text: str,
    ec: str = "M",
    mode: Optional[str] = None,
    version: Optional[int] = None,
) -> EncodingPlan:
    """Work out mode and version for ``text``, checking any forced values."""
    if ec not in tables.EC_LEVELS:
        raise ValueError(f"invalid EC level: {ec}")
    if mode is None or mode == "auto":
        chosen_mode = detect_mode(text)
    else:
        validate_mode(text, mode)
        chosen_mode = mode
    chosen_version = choose_version(text, chosen_mode, ec, version)
    return EncodingPlan(mode=chosen_mode, version=chosen_version, ec=ec, text=text)
