"""CLI / integration tests (in-process and subprocess)."""
import os
import subprocess
import sys
import xml.dom.minidom

import pytest

from qrgen.cli import main

from .conftest import HAVE_DECODER, decode_one


def test_terminal_output(capsys):
    rc = main(["HELLO CC WORLD"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "\n" in out and len(out.strip()) > 0


def test_svg_output_is_wellformed(tmp_path):
    out = tmp_path / "q.svg"
    rc = main(["HELLO", "-o", str(out)])
    assert rc == 0
    content = out.read_text(encoding="utf-8")
    # Parses as XML and is an <svg> document.
    doc = xml.dom.minidom.parseString(content)
    assert doc.documentElement.tagName == "svg"


def test_png_output(tmp_path):
    pytest.importorskip("PIL")
    out = tmp_path / "q.png"
    rc = main(["12345", "-o", str(out), "-s", "8"])
    assert rc == 0
    assert out.exists() and out.stat().st_size > 0


def test_format_inferred_from_extension(tmp_path):
    out = tmp_path / "q.svg"
    main(["TEST", "-o", str(out)])
    assert out.read_text(encoding="utf-8").lstrip().startswith("<?xml")


def test_forced_version_too_small_errors(capsys):
    rc = main(["A" * 100, "-m", "alphanumeric", "-v", "1", "-e", "H"])
    assert rc == 1
    assert "error" in capsys.readouterr().err.lower()


def test_illegal_char_for_forced_mode_errors(capsys):
    rc = main(["hello", "-m", "numeric"])
    assert rc == 1
    err = capsys.readouterr().err.lower()
    assert "numeric" in err


def test_scale_zero_rejected_for_png(tmp_path, capsys):
    out = tmp_path / "q.png"
    rc = main(["X", "-o", str(out), "-s", "0"])
    assert rc == 1
    assert "scale" in capsys.readouterr().err.lower()


def test_invalid_version_argument(capsys):
    with pytest.raises(SystemExit) as exc:
        main(["X", "-v", "99"])
    assert exc.value.code == 2


def test_subprocess_stdin_numeric():
    proc = subprocess.run(
        [sys.executable, "-m", "qrgen", "-m", "numeric"],
        input="12345678\n",
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    assert proc.returncode == 0
    assert proc.stdout.strip()


@pytest.mark.skipif(not HAVE_DECODER, reason="no decoder available")
def test_subprocess_png_roundtrip(tmp_path):
    pytest.importorskip("PIL")
    out = tmp_path / "cc.png"
    proc = subprocess.run(
        [sys.executable, "-m", "qrgen", "HELLO CC WORLD", "-o", str(out)],
        capture_output=True,
        text=True,
    )
    assert proc.returncode == 0
    with open(out, "rb") as fh:
        assert decode_one(fh.read()) == "HELLO CC WORLD"
