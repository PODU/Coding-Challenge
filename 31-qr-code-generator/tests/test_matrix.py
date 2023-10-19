"""Module-placement and capacity-table known-vector tests."""
import pytest

from qrgen import tables
from qrgen.analyze import make_plan
from qrgen.matrix import Matrix, build_base_matrix


def test_total_codewords_known():
    expected = {1: 26, 2: 44, 3: 70, 4: 100, 5: 134, 6: 172, 7: 196,
                10: 346, 40: 3706}
    for ver, total in expected.items():
        assert tables.total_codewords(ver) == total


def test_data_codewords_v1():
    assert tables.num_data_codewords(1, "L") == 19
    assert tables.num_data_codewords(1, "M") == 16
    assert tables.num_data_codewords(1, "Q") == 13
    assert tables.num_data_codewords(1, "H") == 9


def test_block_structure_v5q():
    # Version 5-Q: 2 blocks of 15 + 2 blocks of 16 data codewords, 18 EC each.
    ecpb, lengths = tables.block_structure(5, "Q")
    assert ecpb == 18
    assert sorted(lengths) == [15, 15, 16, 16]


def test_alignment_positions_known():
    assert tables.alignment_positions(1) == []
    assert tables.alignment_positions(2) == [6, 18]
    assert tables.alignment_positions(7) == [6, 22, 38]
    assert tables.alignment_positions(14) == [6, 26, 46, 66]


def test_finder_patterns_present():
    m = Matrix(1)
    m._place_function_patterns()
    n = m.size
    # The three finder centres are dark; their separators (ring) are light.
    for (r, c) in [(3, 3), (3, n - 4), (n - 4, 3)]:
        assert m.modules[r][c] is True
    # Dark module always set.
    assert m.modules[n - 8][8] is True


def test_timing_pattern_alternation():
    m = Matrix(1)
    m._place_function_patterns()
    n = m.size
    for i in range(8, n - 8):
        assert m.modules[6][i] == (i % 2 == 0)
        assert m.modules[i][6] == (i % 2 == 0)


def test_data_module_count_matches_capacity():
    # Number of placed data bits must equal total codewords*8 + remainder bits.
    for ver in (1, 2, 3, 7, 10):
        m = Matrix(ver)
        m._place_function_patterns()
        free = sum(
            not m.is_function[r][c]
            for r in range(m.size) for c in range(m.size)
        )
        assert free == tables.total_codewords(ver) * 8 + tables.remainder_bits(ver)


def test_function_modules_untouched_by_data():
    plan = make_plan("HELLO CC WORLD", ec="M")
    base = build_base_matrix(plan)
    fresh = Matrix(plan.version)
    fresh._place_function_patterns()
    for r in range(base.size):
        for c in range(base.size):
            if fresh.is_function[r][c]:
                assert base.modules[r][c] == fresh.modules[r][c]


def test_version_info_reserved_only_v7plus():
    m6 = Matrix(6)
    m6._place_function_patterns()
    # version info block region (size-11..size-9) x (0..5) must be free for v<7
    assert not m6.is_function[m6.size - 11][0]
    m7 = Matrix(7)
    m7._place_function_patterns()
    assert m7.is_function[m7.size - 11][0]
