from __future__ import annotations

from pathlib import Path

import pytest

from carousel.cli import main


def test_version_exits_zero(capsys):
    with pytest.raises(SystemExit) as exc:
        main(["--version"])
    assert exc.value.code == 0
    assert "carousel" in capsys.readouterr().out


def test_help_exits_zero(capsys):
    with pytest.raises(SystemExit) as exc:
        main(["--help"])
    assert exc.value.code == 0


def test_end_to_end_happy_path(tmp_path, is_pdf, page_count):
    src = tmp_path / "post.md"
    src.write_text("# A\nbody a\n---\n# B\nbody b\n---\n# C\nbody c", encoding="utf-8")
    out = tmp_path / "post.pdf"
    code = main([str(src), "-o", str(out), "--bg", "#0A1F44", "--fg", "white"])
    assert code == 0
    assert is_pdf(out)
    assert page_count(out) == 3


def test_default_output_path(tmp_path, is_pdf):
    src = tmp_path / "deck.md"
    src.write_text("# A\nbody", encoding="utf-8")
    assert main([str(src)]) == 0
    assert is_pdf(tmp_path / "deck.pdf")


def test_missing_input_file_exits_one(tmp_path, capsys):
    code = main([str(tmp_path / "nope.md")])
    assert code == 1
    assert "not found" in capsys.readouterr().err


def test_empty_input_exits_one_no_partial_pdf(tmp_path, capsys):
    src = tmp_path / "empty.md"
    src.write_text("   \n\n", encoding="utf-8")
    out = tmp_path / "empty.pdf"
    code = main([str(src), "-o", str(out)])
    assert code == 1
    assert "empty" in capsys.readouterr().err.lower()
    assert not out.exists()


def test_invalid_color_exits_one(tmp_path, capsys):
    src = tmp_path / "c.md"
    src.write_text("# A\nbody", encoding="utf-8")
    code = main([str(src), "--bg", "not-a-color"])
    assert code == 1
    assert "error" in capsys.readouterr().err.lower()


def test_missing_image_graceful_via_cli(tmp_path, capsys, is_pdf):
    src = tmp_path / "p.md"
    src.write_text("# A\nbody", encoding="utf-8")
    out = tmp_path / "p.pdf"
    code = main([str(src), "-o", str(out), "--profile-image", str(tmp_path / "ghost.png")])
    assert code == 0
    assert "profile image not found" in capsys.readouterr().err
    assert is_pdf(out)


def test_config_file_auto_discovered(tmp_path, make_png, is_pdf, page_count):
    (tmp_path / "style.json").write_text(
        '{"style": {"background": "#101010", "font_size": 44}, '
        '"profile": {"name": "Jane", "handle": "@jane"}}',
        encoding="utf-8",
    )
    src = tmp_path / "post.md"
    src.write_text("# A\nbody a\n---\n# B\nbody b", encoding="utf-8")
    out = tmp_path / "post.pdf"
    assert main([str(src), "-o", str(out)]) == 0
    assert is_pdf(out)
    assert page_count(out) == 2


def test_cli_override_beats_config(tmp_path):
    # CLI --size should win over a config size; clamping still applies.
    (tmp_path / "style.json").write_text('{"style": {"size": 1080}}', encoding="utf-8")
    src = tmp_path / "post.md"
    src.write_text("# A\nbody", encoding="utf-8")
    out = tmp_path / "post.pdf"
    assert main([str(src), "-o", str(out), "--size", "1200", "-q"]) == 0
    # 1200px square page == 1200pt in the PDF mediabox.
    from pypdf import PdfReader

    box = PdfReader(str(out)).pages[0].mediabox
    assert round(float(box.width)) == 1200
