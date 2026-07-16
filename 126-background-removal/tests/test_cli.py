"""End-to-end CLI tests, exit codes included (Steps 5, 6, 7, 8, 9)."""

from __future__ import annotations

from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw

from ccbgremove.cli import main


def _write_portrait(path: Path, size: int = 60) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    img = Image.new("RGB", (size, size), (40, 40, 40))
    ImageDraw.Draw(img).ellipse(
        [10, 10, size - 10, size - 10], fill=(230, 200, 180)
    )
    img.save(path)


def test_single_file_default_output(tmp_path: Path):
    src = tmp_path / "me.jpg"
    _write_portrait(src)
    rc = main([str(src), "--backend", "none"])
    assert rc == 0
    out = tmp_path / "me-nobg.png"
    assert out.exists()
    with Image.open(out) as im:
        assert im.mode == "RGBA"


def test_single_file_explicit_output(tmp_path: Path):
    src = tmp_path / "me.png"
    _write_portrait(src)
    dst = tmp_path / "cut.png"
    rc = main([str(src), "-o", str(dst), "--backend", "none"])
    assert rc == 0 and dst.exists()


def test_invalid_file_nonzero_exit(tmp_path: Path):
    bad = tmp_path / "broken.jpg"
    bad.write_text("garbage")
    rc = main([str(bad), "--backend", "none"])
    assert rc == 1
    assert not (tmp_path / "broken-nobg.png").exists()


def test_missing_input_nonzero_exit(tmp_path: Path):
    rc = main([str(tmp_path / "ghost.png"), "--backend", "none"])
    assert rc == 1


def test_directory_mirrored_output(tmp_path: Path):
    root = tmp_path / "in"
    _write_portrait(root / "a.jpg")
    _write_portrait(root / "nested/b.png")
    out = tmp_path / "out"
    rc = main([str(root), "-o", str(out), "--backend", "none"])
    assert rc == 0
    # Directory structure is mirrored.
    assert (out / "a-nobg.png").exists()
    assert (out / "nested/b-nobg.png").exists()


def test_batch_continues_and_reports_failure(tmp_path: Path):
    root = tmp_path / "in"
    _write_portrait(root / "good.jpg")
    (root / "bad.png").write_bytes(b"not a png")
    out = tmp_path / "out"
    rc = main([str(root), "-o", str(out), "--backend", "none"])
    # One failure gives a non-zero exit, but the good file is still written.
    assert rc == 1
    assert (out / "good-nobg.png").exists()


def test_grabcut_end_to_end_produces_transparency(tmp_path: Path):
    src = tmp_path / "portrait.jpg"
    _write_portrait(src, size=100)
    rc = main([str(src), "--backend", "grabcut", "--hard-edges"])
    assert rc == 0
    with Image.open(tmp_path / "portrait-nobg.png") as im:
        alpha = np.asarray(im)[:, :, 3]
    # Some fully transparent (background) and some fully opaque (subject).
    assert alpha.min() == 0
    assert alpha.max() == 255
