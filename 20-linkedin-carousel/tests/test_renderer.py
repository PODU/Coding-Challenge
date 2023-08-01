from __future__ import annotations

from pathlib import Path

from carousel.models import Carousel, Profile, Slide, Style
from carousel.parser import parse_document
from carousel.renderer import render_carousel
from pypdf import PdfReader


def _carousel(slides, **style_kw):
    return Carousel(style=Style(**style_kw), slides=slides)


def test_render_three_slides_valid_pdf(tmp_path, is_pdf, page_count):
    text = "# A\nbody a\n---\n# B\nbody b\n---\n# C\nbody c"
    slides = parse_document(text)
    out = tmp_path / "out.pdf"
    render_carousel(_carousel(slides), str(out))
    assert is_pdf(out)
    assert page_count(out) == 3


def test_page_count_matches_slide_count(tmp_path, page_count):
    for n in (1, 2, 5, 10):
        slides = parse_document("\n---\n".join(f"# S{i}\nbody {i}" for i in range(n)))
        out = tmp_path / f"n{n}.pdf"
        render_carousel(_carousel(slides), str(out))
        assert page_count(out) == n


def test_render_with_background_and_profile_images(tmp_path, make_png, page_count):
    bg = make_png(tmp_path / "bg.png", color=(20, 80, 160))
    pic = make_png(tmp_path / "me.png", color=(240, 240, 240))
    slide = Slide(title="Hi", body_lines=["with images"], background_image=str(bg))
    carousel = Carousel(
        profile=Profile(image=str(pic), name="Jane", handle="@jane"),
        style=Style(),
        slides=[slide],
    )
    out = tmp_path / "img.pdf"
    render_carousel(carousel, str(out))
    assert page_count(out) == 1
    assert out.stat().st_size > 0


def test_missing_profile_image_degrades_gracefully(tmp_path, capsys, is_pdf, page_count):
    carousel = Carousel(
        profile=Profile(image=str(tmp_path / "nope.png"), name="Jane"),
        style=Style(),
        slides=[Slide(title="T", body_lines=["body"])],
    )
    out = tmp_path / "missing.pdf"
    render_carousel(carousel, str(out))
    err = capsys.readouterr().err
    assert "profile image not found" in err
    assert is_pdf(out)
    assert page_count(out) == 1


def test_missing_background_image_warns_and_continues(tmp_path, capsys, page_count):
    slide = Slide(body_lines=["x"], background_image=str(tmp_path / "absent.jpg"))
    out = tmp_path / "bgmiss.pdf"
    render_carousel(_carousel([slide]), str(out))
    assert "background image not found" in capsys.readouterr().err
    assert page_count(out) == 1


def test_long_text_wraps_without_crash(tmp_path, page_count):
    body = ("This is a very long line of text that should wrap and shrink. " * 60).strip()
    out = tmp_path / "long.pdf"
    render_carousel(_carousel([Slide(title="Long", body_lines=[body])]), str(out))
    assert page_count(out) == 1


def test_missing_font_falls_back(tmp_path, capsys, is_pdf):
    out = tmp_path / "font.pdf"
    render_carousel(_carousel([Slide(body_lines=["hi"])], font="NoSuchFont.ttf"), str(out))
    assert "font" in capsys.readouterr().err.lower()
    assert is_pdf(out)


def test_quiet_suppresses_warnings(tmp_path, capsys):
    carousel = Carousel(
        profile=Profile(image=str(tmp_path / "nope.png")),
        style=Style(),
        slides=[Slide(body_lines=["x"])],
    )
    render_carousel(carousel, str(tmp_path / "q.pdf"), quiet=True)
    assert capsys.readouterr().err == ""


def test_no_network(monkeypatch, tmp_path, page_count):
    import socket

    def _blocked(self, *a, **k):
        raise AssertionError("network access attempted during render")

    # Any attempt to open a socket connection fails the test.
    monkeypatch.setattr(socket.socket, "connect", _blocked, raising=False)
    out = tmp_path / "offline.pdf"
    render_carousel(_carousel([Slide(body_lines=["offline"])]), str(out))
    assert page_count(out) == 1


def test_determinism_same_text(tmp_path, page_count):
    slides = parse_document("# A\nbody a\n---\n# B\nbody b")
    out1 = tmp_path / "d1.pdf"
    out2 = tmp_path / "d2.pdf"
    render_carousel(_carousel(slides), str(out1))
    render_carousel(_carousel(slides), str(out2))
    assert page_count(out1) == page_count(out2)
    t1 = [p.extract_text() for p in PdfReader(str(out1)).pages]
    t2 = [p.extract_text() for p in PdfReader(str(out2)).pages]
    assert t1 == t2


def test_oversize_clamped_renders(tmp_path, page_count):
    style = Style(size=9000)
    assert style.size == 4320
    render_carousel(Carousel(style=style, slides=[Slide(body_lines=["x"])]), str(tmp_path / "big.pdf"))
    assert page_count(tmp_path / "big.pdf") == 1
