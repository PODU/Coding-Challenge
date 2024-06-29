"""Shared fixtures: build the archives and wordlists the suite cracks.

Everything here is deterministic and self-contained - no network, no external
``zip`` binary. The ZipCrypto fixture is produced by our own minimal writer
(``_zipcrypto_writer``) because neither the stdlib nor pyzipper can *write* the
legacy stream cipher. The AES fixture uses pyzipper and is gated on its import.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent))
from _zipcrypto_writer import build_zipcrypto_zip  # noqa: E402

PASSWORD = "test"
PLAINTEXT = b"correct horse battery staple\n"
ENTRY_NAME = "cc.txt"

try:
    import pyzipper  # type: ignore[import-untyped]

    HAVE_PYZIPPER = True
except ImportError:  # pragma: no cover
    HAVE_PYZIPPER = False

requires_pyzipper = pytest.mark.skipif(
    not HAVE_PYZIPPER, reason="pyzipper not installed; AES path skipped"
)


@pytest.fixture
def zipcrypto_zip(tmp_path: Path) -> Path:
    """A single-entry ZipCrypto archive, password 'test'."""
    path = tmp_path / "cctest.zip"
    path.write_bytes(
        build_zipcrypto_zip(ENTRY_NAME, PLAINTEXT, PASSWORD, seed=42)
    )
    return path


@pytest.fixture
def plain_zip(tmp_path: Path) -> Path:
    """An unencrypted ZIP archive (for the 'not encrypted' branch)."""
    import zipfile

    path = tmp_path / "plain.zip"
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr(ENTRY_NAME, PLAINTEXT)
    return path


@pytest.fixture
def aes_zip(tmp_path: Path) -> Path:
    """A WinZip AES-256 archive, password 'test' (requires pyzipper)."""
    if not HAVE_PYZIPPER:
        pytest.skip("pyzipper not installed")
    import pyzipper

    path = tmp_path / "aes.zip"
    with pyzipper.AESZipFile(
        path, "w", compression=pyzipper.ZIP_DEFLATED, encryption=pyzipper.WZ_AES
    ) as zf:
        zf.setpassword(PASSWORD.encode("utf-8"))
        zf.writestr(ENTRY_NAME, PLAINTEXT)
    return path


@pytest.fixture
def non_zip(tmp_path: Path) -> Path:
    """A plain text file that is not a ZIP at all."""
    path = tmp_path / "cc.txt"
    path.write_bytes(b"this is not a zip file\n")
    return path


@pytest.fixture
def tiny_wordlist(tmp_path: Path) -> Path:
    """A short wordlist that contains the password 'test'."""
    path = tmp_path / "tiny.txt"
    path.write_text("apple\nbanana\ntest\ncherry\n", encoding="utf-8")
    return path


@pytest.fixture
def nomatch_wordlist(tmp_path: Path) -> Path:
    """A short wordlist that does NOT contain the password."""
    path = tmp_path / "nomatch.txt"
    path.write_text("apple\nbanana\ncherry\ndurian\n", encoding="utf-8")
    return path
