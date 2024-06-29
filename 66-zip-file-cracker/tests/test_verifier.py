"""Per-candidate verification (challenge Step 2/3 primitive)."""

from __future__ import annotations

from pathlib import Path

from cczipcrack.verifier import Verifier
from conftest import requires_pyzipper


def test_verify_correct_password(zipcrypto_zip: Path) -> None:
    with Verifier(str(zipcrypto_zip), "zipcrypto", "cc.txt") as v:
        assert v.verify("test") is True


def test_verify_wrong_password(zipcrypto_zip: Path) -> None:
    with Verifier(str(zipcrypto_zip), "zipcrypto", "cc.txt") as v:
        assert v.verify("wrong") is False


def test_verify_is_reusable(zipcrypto_zip: Path) -> None:
    # The same open handle must serve many candidates, wrong then right.
    with Verifier(str(zipcrypto_zip), "zipcrypto", "cc.txt") as v:
        assert v.verify("nope") is False
        assert v.verify("also-no") is False
        assert v.verify("test") is True


@requires_pyzipper
def test_verify_aes(aes_zip: Path) -> None:
    with Verifier(str(aes_zip), "aes", "cc.txt") as v:
        assert v.verify("wrong") is False
        assert v.verify("test") is True
