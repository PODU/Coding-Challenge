"""CLI behaviour: exact transcript strings and exit codes."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest

from cczipcrack import cli
from conftest import requires_pyzipper


def test_validate_only_has_headers(
    zipcrypto_zip: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main([str(zipcrypto_zip)])
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_OK
    assert out == "Has zip headers"


def test_validate_only_no_headers(
    non_zip: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main([str(non_zip)])
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_USAGE
    assert out == "Does not have zip headers"


def test_missing_file(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main([str(tmp_path / "nope.zip")])
    assert rc == cli.EXIT_USAGE
    assert "no such file" in capsys.readouterr().out


def test_dict_success(
    zipcrypto_zip: Path,
    tiny_wordlist: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    rc = cli.main(
        ["-dict", "-w", str(tiny_wordlist), "-j", "1", str(zipcrypto_zip)]
    )
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_OK
    assert out == "Password found: test"


def test_dict_miss(
    zipcrypto_zip: Path,
    nomatch_wordlist: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    rc = cli.main(
        ["-dict", "-w", str(nomatch_wordlist), "-j", "1", str(zipcrypto_zip)]
    )
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_NOT_FOUND
    assert out == "Password not found"


def test_brute_success(
    zipcrypto_zip: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main(
        [
            "-brute",
            "-min=1",
            "-max=4",
            "--charset=set",
            "-j",
            "1",
            str(zipcrypto_zip),
        ]
    )
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_OK
    assert out == "Password found: test"


def test_not_encrypted_branch(
    plain_zip: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main(["-dict", "-w", "whatever.txt", "-j", "1", str(plain_zip)])
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_OK
    assert out == "Archive is not encrypted; nothing to crack"


def test_missing_wordlist(
    zipcrypto_zip: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = cli.main(
        ["-dict", "-w", "does-not-exist.txt", "-j", "1", str(zipcrypto_zip)]
    )
    assert rc == cli.EXIT_USAGE


@requires_pyzipper
def test_aes_crack(
    aes_zip: Path,
    tiny_wordlist: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    rc = cli.main(
        ["-dict", "-w", str(tiny_wordlist), "-j", "1", str(aes_zip)]
    )
    out = capsys.readouterr().out.strip()
    assert rc == cli.EXIT_OK
    assert out == "Password found: test"


def test_subprocess_smoke(zipcrypto_zip: Path, tiny_wordlist: Path) -> None:
    """End-to-end via `python -m cczipcrack`, matching the challenge transcript."""
    proc = subprocess.run(
        [
            sys.executable,
            "-m",
            "cczipcrack",
            "-dict",
            "-w",
            str(tiny_wordlist),
            "-j",
            "1",
            str(zipcrypto_zip),
        ],
        capture_output=True,
        text=True,
    )
    assert proc.returncode == 0
    assert proc.stdout.strip() == "Password found: test"
