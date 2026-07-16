"""Path planning and batch tests (Steps 5, 7, 8, 9)."""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest
from PIL import Image

from ccbgremove.errors import OutputCollisionError
from ccbgremove.pipeline import (
    Job,
    default_output_path,
    iter_images,
    mirror_output_path,
    process_image,
    process_jobs,
)
from ccbgremove.segment import OpaqueSegmenter


def _write_portrait(path: Path, size: int = 60) -> None:
    from PIL import ImageDraw

    path.parent.mkdir(parents=True, exist_ok=True)
    img = Image.new("RGB", (size, size), (40, 40, 40))
    d = ImageDraw.Draw(img)
    d.ellipse([10, 10, size - 10, size - 10], fill=(230, 200, 180))
    img.save(path)


def test_default_output_never_overwrites_input():
    src = Path("/photos/holiday.png")
    dst = default_output_path(src)
    assert dst != src
    assert dst.suffix == ".png"
    assert dst.name == "holiday-nobg.png"


def test_default_output_into_dir():
    dst = default_output_path(Path("a/b/c.jpg"), out_dir=Path("out"))
    assert dst == Path("out/c-nobg.png")


def test_process_refuses_to_overwrite_input(png_file: Path):
    job = Job(src=png_file, dst=png_file)
    with pytest.raises(OutputCollisionError):
        process_image(job, OpaqueSegmenter())


def test_mirror_output_path_preserves_structure():
    src = Path("in/people/group/a.jpg")
    dst = mirror_output_path(src, Path("in"), Path("out"))
    assert dst == Path("out/people/group/a-nobg.png")


def test_iter_images_recursive(tmp_path: Path):
    _write_portrait(tmp_path / "a.jpg")
    _write_portrait(tmp_path / "sub/b.png")
    (tmp_path / "notes.txt").write_text("skip me")
    found = list(iter_images(tmp_path, recursive=True))
    names = {p.name for p in found}
    assert names == {"a.jpg", "b.png"}


def test_iter_images_non_recursive(tmp_path: Path):
    _write_portrait(tmp_path / "a.jpg")
    _write_portrait(tmp_path / "sub/b.png")
    found = list(iter_images(tmp_path, recursive=False))
    assert {p.name for p in found} == {"a.jpg"}


def test_process_jobs_continues_on_failure(tmp_path: Path):
    good = tmp_path / "good.png"
    _write_portrait(good)
    bad = tmp_path / "bad.jpg"
    bad.write_text("not an image")

    out = tmp_path / "out"
    jobs = [
        Job(src=good, dst=out / "good-nobg.png"),
        Job(src=bad, dst=out / "bad-nobg.png"),
    ]
    result = process_jobs(jobs, OpaqueSegmenter())

    assert len(result.successes) == 1
    assert len(result.failures) == 1
    assert result.failures[0][0] == bad
    assert not result.ok
    assert (out / "good-nobg.png").exists()


def test_process_image_writes_rgba_same_size(tmp_path: Path):
    src = tmp_path / "s.png"
    _write_portrait(src, size=80)
    dst = tmp_path / "out.png"
    process_image(Job(src=src, dst=dst), OpaqueSegmenter())
    with Image.open(dst) as im:
        assert im.mode == "RGBA"
        assert im.size == (80, 80)
