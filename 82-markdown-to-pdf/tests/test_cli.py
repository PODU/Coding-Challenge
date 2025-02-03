"""CLI tests: argument handling, IO, exit codes, clean error messages."""

from __future__ import annotations

import os

from conftest import assert_valid_pdf

from md2pdf import cli


def test_default_output_path(tmp_path, monkeypatch):
    src = tmp_path / "foo.md"
    src.write_text("# Title\n\nBody.\n", encoding="utf-8")
    monkeypatch.chdir(tmp_path)
    rc = cli.main([str(src), "-q"])
    assert rc == 0
    out = tmp_path / "foo.pdf"
    assert out.exists()
    assert_valid_pdf(out.read_bytes())


def test_output_flag_honoured(tmp_path):
    src = tmp_path / "in.md"
    src.write_text("# Hi\n", encoding="utf-8")
    out = tmp_path / "sub" / "out.pdf"
    out.parent.mkdir()
    rc = cli.main([str(src), "-o", str(out), "-q"])
    assert rc == 0 and out.exists()


def test_stdin_input(tmp_path, monkeypatch):
    out = tmp_path / "o.pdf"
    import io
    import sys

    class _Buf:
        buffer = io.BytesIO(b"# From stdin\n")

    monkeypatch.setattr(sys, "stdin", _Buf())
    rc = cli.main(["-", "-o", str(out), "-q"])
    assert rc == 0 and out.exists()
    assert_valid_pdf(out.read_bytes())


def test_page_size_and_font_flags(tmp_path):
    src = tmp_path / "in.md"
    src.write_text("# Hi\n\ntext\n", encoding="utf-8")
    out = tmp_path / "out.pdf"
    rc = cli.main(
        [str(src), "-o", str(out), "--page-size", "Letter", "--font", "Times-Roman",
         "--font-size", "12", "--heading-scale", "1.15", "-q"]
    )
    assert rc == 0 and out.exists()


def test_missing_input_exit_2(tmp_path, capsys):
    rc = cli.main([str(tmp_path / "nope.md"), "-q"])
    assert rc == 2
    err = capsys.readouterr().err
    assert "not found" in err
    assert "Traceback" not in err


def test_bad_flag_exit_1(capsys):
    import pytest

    with pytest.raises(SystemExit) as exc:
        cli.main(["x.md", "--page-size", "Foolscap"])
    assert exc.value.code == 2  # argparse uses exit code 2 for usage errors
    assert "invalid choice" in capsys.readouterr().err


def test_bad_hex_colour_exit(capsys):
    import pytest

    with pytest.raises(SystemExit):
        cli.main(["x.md", "--background", "notacolour"])
    assert "invalid hex" in capsys.readouterr().err


def test_unwritable_output_exit_2(tmp_path, capsys):
    src = tmp_path / "in.md"
    src.write_text("# Hi\n", encoding="utf-8")
    # A directory path as output is not writable as a file.
    bad_out = tmp_path / "adir"
    bad_out.mkdir()
    rc = cli.main([str(src), "-o", str(bad_out), "-q"])
    assert rc == 2
    assert "cannot write output" in capsys.readouterr().err


def test_version(capsys):
    import pytest

    with pytest.raises(SystemExit) as exc:
        cli.main(["--version"])
    assert exc.value.code == 0
    assert "md2pdf" in capsys.readouterr().out


def test_help(capsys):
    import pytest

    with pytest.raises(SystemExit) as exc:
        cli.main(["--help"])
    assert exc.value.code == 0
    assert "INPUT.md" in capsys.readouterr().out
