"""Mode detection, bit encoding, char-count, and padding tests."""
import pytest

from qrgen import tables
from qrgen.analyze import (
    CapacityError,
    ModeError,
    detect_mode,
    make_plan,
)
from qrgen.encode import BitBuffer, encode_data_codewords, encode_segment


def bits_str(buf: BitBuffer) -> str:
    return "".join(str(b) for b in buf.bits)


def test_mode_detection():
    assert detect_mode("12345") == tables.MODE_NUMERIC
    assert detect_mode("HELLO WORLD") == tables.MODE_ALPHANUMERIC
    assert detect_mode("HELLO CC WORLD") == tables.MODE_ALPHANUMERIC
    assert detect_mode("hello") == tables.MODE_BYTE          # lowercase -> byte
    assert detect_mode("https://a.com") == tables.MODE_BYTE  # '/'+lowercase
    assert detect_mode("café") == tables.MODE_BYTE           # latin-1


def test_numeric_grouping_remainders():
    # 8 digits -> two groups of 3 (10 bits each) + group of 2 (7 bits).
    plan = make_plan("12345678", ec="L", mode="numeric")
    seg = encode_segment(plan)
    # mode(4) + count(10) + 10 + 10 + 7
    assert len(seg) == 4 + 10 + 10 + 10 + 7
    # single trailing digit -> 4-bit group
    plan1 = make_plan("1234567", ec="L", mode="numeric")
    seg1 = encode_segment(plan1)
    assert len(seg1) == 4 + 10 + 10 + 10 + 4


def test_alphanumeric_pairs_and_lone_char():
    plan = make_plan("ABCDE", ec="L", mode="alphanumeric")
    seg = encode_segment(plan)
    # mode(4) + count(9) + 11 + 11 + 6 (two pairs + lone)
    assert len(seg) == 4 + 9 + 11 + 11 + 6


def test_hello_world_bitstring():
    # Thonky worked example header + data bits for "HELLO WORLD" alphanumeric.
    plan = make_plan("HELLO WORLD", ec="Q")
    seg = encode_segment(plan)
    s = bits_str(seg)
    # mode 0010, count = 11 = 000001011 (9 bits), then 11-bit groups.
    assert s.startswith("0010" + "000001011")
    # HE -> 45*17+14 = 779 -> 01100001011
    assert s[13:24] == "01100001011"


def test_padding_alternates_ec_11():
    plan = make_plan("1", ec="M", mode="numeric")  # tiny payload, lots of pad
    cw = encode_data_codewords(plan)
    assert len(cw) == tables.num_data_codewords(1, "M")
    pads = cw[cw.index(0xEC):]
    assert pads[0] == 0xEC and pads[1] == 0x11 and pads[2] == 0xEC


def test_byte_mode_latin1():
    plan = make_plan("hello", ec="M")
    assert plan.mode == "byte"
    seg = encode_segment(plan)
    assert len(seg) == 4 + 8 + 5 * 8


def test_forced_mode_rejects_illegal_char():
    with pytest.raises(ModeError):
        make_plan("hello", mode="numeric")
    with pytest.raises(ModeError):
        make_plan("hello", mode="alphanumeric")


def test_capacity_overflow_raises():
    with pytest.raises(CapacityError):
        make_plan("8" * 8000, ec="H", mode="numeric")


def test_version_boundary_selection():
    # Char-count width changes at version 9->10 and 26->27; selection must
    # still find the smallest fitting version.
    plan = make_plan("A" * 100, ec="L", mode="alphanumeric")
    assert plan.version >= 1
    # Force a version that cannot hold the data -> error.
    with pytest.raises(CapacityError):
        make_plan("A" * 100, ec="H", mode="alphanumeric", version=1)


def test_char_count_widths():
    assert tables.char_count_bits("numeric", 1) == 10
    assert tables.char_count_bits("numeric", 10) == 12
    assert tables.char_count_bits("numeric", 27) == 14
    assert tables.char_count_bits("byte", 1) == 8
    assert tables.char_count_bits("byte", 10) == 16
    assert tables.char_count_bits("alphanumeric", 9) == 9
    assert tables.char_count_bits("alphanumeric", 10) == 11
