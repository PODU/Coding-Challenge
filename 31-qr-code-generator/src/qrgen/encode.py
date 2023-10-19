"""Build the codeword stream: header, data bits, padding and error correction."""
from __future__ import annotations

from typing import List

from . import reed_solomon, tables
from .analyze import EncodingPlan
from .tables import (
    ALPHANUMERIC_MAP,
    MODE_ALPHANUMERIC,
    MODE_BYTE,
    MODE_INDICATOR,
    MODE_KANJI,
    MODE_NUMERIC,
)

PAD_BYTES = (0xEC, 0x11)


class BitBuffer:
    """Bits appended most significant first."""

    def __init__(self) -> None:
        self.bits: List[int] = []

    def append(self, value: int, length: int) -> None:
        if value >> length:
            raise ValueError(f"value {value} does not fit in {length} bits")
        for i in range(length - 1, -1, -1):
            self.bits.append((value >> i) & 1)

    def __len__(self) -> int:
        return len(self.bits)

    def to_bytes(self) -> bytes:
        if len(self.bits) % 8 != 0:
            raise ValueError("bit length is not a byte multiple")
        out = bytearray()
        for i in range(0, len(self.bits), 8):
            byte = 0
            for b in self.bits[i:i + 8]:
                byte = (byte << 1) | b
            out.append(byte)
        return bytes(out)


def byte_payload(text: str) -> bytes:
    """Latin-1 if the text allows it, else UTF-8."""
    try:
        return text.encode("latin-1")
    except UnicodeEncodeError:
        return text.encode("utf-8")


def data_bit_length(text: str, mode: str) -> int:
    """Bits for the data alone, without the header or padding."""
    if mode == MODE_NUMERIC:
        full, rem = divmod(len(text), 3)
        return full * 10 + [0, 4, 7][rem]
    if mode == MODE_ALPHANUMERIC:
        n = len(text)
        return (n // 2) * 11 + (6 if n % 2 else 0)
    if mode == MODE_BYTE:
        return len(byte_payload(text)) * 8
    if mode == MODE_KANJI:
        return len(text) * 13
    raise ValueError(f"unknown mode: {mode}")


def _encode_numeric(text: str, buf: BitBuffer) -> None:
    for i in range(0, len(text), 3):
        chunk = text[i:i + 3]
        buf.append(int(chunk), [0, 4, 7, 10][len(chunk)])


def _encode_alphanumeric(text: str, buf: BitBuffer) -> None:
    i = 0
    while i + 1 < len(text):
        v = ALPHANUMERIC_MAP[text[i]] * 45 + ALPHANUMERIC_MAP[text[i + 1]]
        buf.append(v, 11)
        i += 2
    if i < len(text):
        buf.append(ALPHANUMERIC_MAP[text[i]], 6)


def _encode_byte(text: str, buf: BitBuffer) -> None:
    for b in byte_payload(text):
        buf.append(b, 8)


def _encode_kanji(text: str, buf: BitBuffer) -> None:
    raw = text.encode("shift_jis")
    for i in range(0, len(raw), 2):
        word = (raw[i] << 8) | raw[i + 1]
        if 0x8140 <= word <= 0x9FFC:
            word -= 0x8140
        else:  # 0xE040..0xEBBF
            word -= 0xC140
        value = ((word >> 8) * 0xC0) + (word & 0xFF)
        buf.append(value, 13)


def _char_count(text: str, mode: str) -> int:
    if mode == MODE_BYTE:
        return len(byte_payload(text))
    return len(text)


def encode_segment(plan: EncodingPlan) -> BitBuffer:
    """Mode indicator, character count and data, without padding."""
    buf = BitBuffer()
    buf.append(MODE_INDICATOR[plan.mode], 4)
    count_bits = tables.char_count_bits(plan.mode, plan.version)
    buf.append(_char_count(plan.text, plan.mode), count_bits)
    encoder = {
        MODE_NUMERIC: _encode_numeric,
        MODE_ALPHANUMERIC: _encode_alphanumeric,
        MODE_BYTE: _encode_byte,
        MODE_KANJI: _encode_kanji,
    }[plan.mode]
    encoder(plan.text, buf)
    return buf


def encode_data_codewords(plan: EncodingPlan) -> List[int]:
    """Data codewords: the segment, a terminator, then padding to capacity."""
    capacity_bits = tables.data_capacity_bits(plan.version, plan.ec)
    buf = encode_segment(plan)
    if len(buf) > capacity_bits:
        raise ValueError("encoded segment exceeds capacity (internal error)")

    # Up to four zero bits of terminator, then zeros to the next byte.
    buf.append(0, min(4, capacity_bits - len(buf)))
    if len(buf) % 8 != 0:
        buf.append(0, 8 - (len(buf) % 8))

    codewords = list(buf.to_bytes())

    # Fill the rest with the alternating pad bytes 0xEC, 0x11.
    capacity_bytes = capacity_bits // 8
    idx = 0
    while len(codewords) < capacity_bytes:
        codewords.append(PAD_BYTES[idx % 2])
        idx += 1
    return codewords


def split_blocks(data_codewords: List[int], plan: EncodingPlan):
    """Split the data into blocks; return ``(data_blocks, ec_blocks)``."""
    ecpb, lengths = tables.block_structure(plan.version, plan.ec)
    data_blocks: List[List[int]] = []
    ec_blocks: List[List[int]] = []
    pos = 0
    for length in lengths:
        block = data_codewords[pos:pos + length]
        pos += length
        data_blocks.append(block)
        ec_blocks.append(reed_solomon.ec_codewords(block, ecpb))
    return data_blocks, ec_blocks


def interleave(data_blocks, ec_blocks) -> List[int]:
    """Take one codeword from each block in turn, data blocks first."""
    result: List[int] = []
    max_data = max(len(b) for b in data_blocks)
    for i in range(max_data):
        for block in data_blocks:
            if i < len(block):
                result.append(block[i])
    max_ec = max(len(b) for b in ec_blocks)
    for i in range(max_ec):
        for block in ec_blocks:
            if i < len(block):
                result.append(block[i])
    return result


def final_bitstream(plan: EncodingPlan) -> List[int]:
    """All bits to place in the matrix, including the remainder bits."""
    data_codewords = encode_data_codewords(plan)
    data_blocks, ec_blocks = split_blocks(data_codewords, plan)
    codewords = interleave(data_blocks, ec_blocks)
    bits: List[int] = []
    for cw in codewords:
        for i in range(7, -1, -1):
            bits.append((cw >> i) & 1)
    bits.extend([0] * tables.remainder_bits(plan.version))
    return bits
