"""Golden-matrix tests: compare our data region against the qrcode library.

The reference `qrcode` library is configured with the same version/EC/mask, so
the *data + EC module region* (everything except mask-numbering-specific format
bits) must match bit-for-bit. This isolates our placement/masking from any
decoder leniency.
"""
import pytest

from .conftest import HAVE_QRCODE

pytestmark = pytest.mark.skipif(not HAVE_QRCODE, reason="qrcode lib unavailable")

if HAVE_QRCODE:
    import qrcode

    from qrgen import encode as our_encode
    from qrgen.matrix import Matrix

    EC_MAP = {
        "L": qrcode.constants.ERROR_CORRECT_L,
        "M": qrcode.constants.ERROR_CORRECT_M,
        "Q": qrcode.constants.ERROR_CORRECT_Q,
        "H": qrcode.constants.ERROR_CORRECT_H,
    }


def _function_map(version: int):
    m = Matrix(version)
    m._place_function_patterns()
    return m.is_function


@pytest.mark.parametrize(
    "text,ec",
    [
        ("HELLO CC WORLD", "M"),
        ("HELLO WORLD", "Q"),
        ("THE QUICK BROWN FOX", "L"),
        ("123456789012", "H"),
    ],
)
def test_data_region_matches_qrcode(text, ec):
    ours = our_encode(text, ec=ec)
    ref = qrcode.QRCode(
        version=ours.version,
        error_correction=EC_MAP[ec],
        mask_pattern=ours.mask,
        border=0,
    )
    ref.add_data(text)
    ref.make(fit=False)
    refm = ref.get_matrix()
    assert len(refm) == ours.size

    fn = _function_map(ours.version)
    # Non-function (data + EC) modules must match exactly under the same mask.
    mismatches = [
        (r, c)
        for r in range(ours.size)
        for c in range(ours.size)
        if not fn[r][c] and bool(refm[r][c]) != bool(ours.modules[r][c])
    ]
    assert mismatches == []
