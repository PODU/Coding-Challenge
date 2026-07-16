"""Turn inputs into output paths and process them one by one."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from . import image_io
from .errors import BgRemoveError, OutputCollisionError
from .refine import build_output
from .segment import Segmenter

DEFAULT_SUFFIX = "-nobg"


@dataclass
class Job:
    src: Path
    dst: Path


@dataclass
class BatchResult:
    successes: list[Path] = field(default_factory=list)
    failures: list[tuple[Path, str]] = field(default_factory=list)

    @property
    def ok(self) -> bool:
        return not self.failures


def default_output_path(
    src: Path, out_dir: Path | None = None, suffix: str = DEFAULT_SUFFIX
) -> Path:
    """Return ``<stem><suffix>.png`` next to *src*, or in *out_dir* if given.

    The suffix keeps the output from overwriting a PNG input with the same stem.
    """
    parent = out_dir if out_dir is not None else src.parent
    return parent / f"{src.stem}{suffix}.png"


def mirror_output_path(
    src: Path, in_root: Path, out_root: Path, suffix: str = DEFAULT_SUFFIX
) -> Path:
    """Map *src* under *in_root* to the same relative place under *out_root*."""
    rel = src.relative_to(in_root)
    return out_root / rel.parent / f"{src.stem}{suffix}.png"


def iter_images(root: Path, recursive: bool = True):
    """Yield the JPG/PNG files under *root* in sorted order."""
    walker = root.rglob("*") if recursive else root.iterdir()
    for path in sorted(walker):
        if path.is_file() and image_io.is_supported(path):
            yield path


def process_image(
    job: Job,
    segmenter: Segmenter,
    feather: float = 2.0,
    hard: bool = False,
    overwrite: bool = False,
) -> None:
    """Cut out one image. Refuses to overwrite the source unless *overwrite*."""
    src, dst = job.src, job.dst

    try:
        if src.resolve() == dst.resolve() and not overwrite:
            raise OutputCollisionError(
                f"{dst}: output would overwrite the input; refusing "
                f"(use --overwrite to force)"
            )
    except OSError:
        # resolve() can fail on odd paths; fall back to a plain comparison.
        if src == dst and not overwrite:
            raise OutputCollisionError(f"{dst}: output would overwrite the input")

    rgba = image_io.load_rgba(src)
    mask = segmenter.segment(rgba[:, :, :3])
    out = build_output(rgba, mask, feather=feather, hard=hard)
    image_io.save_png(dst, out)


def process_jobs(
    jobs: list[Job],
    segmenter: Segmenter,
    feather: float = 2.0,
    hard: bool = False,
    overwrite: bool = False,
    on_event=None,
) -> BatchResult:
    """Run every job, carrying on after failures.

    *on_event*, if given, is called as ``on_event(kind, src, detail)`` with
    ``kind`` either ``"ok"`` or ``"fail"``.
    """
    result = BatchResult()
    for job in jobs:
        try:
            process_image(
                job, segmenter, feather=feather, hard=hard, overwrite=overwrite
            )
        except (BgRemoveError, OSError, ValueError) as exc:
            result.failures.append((job.src, str(exc)))
            if on_event:
                on_event("fail", job.src, str(exc))
        else:
            result.successes.append(job.dst)
            if on_event:
                on_event("ok", job.src, str(job.dst))
    return result
