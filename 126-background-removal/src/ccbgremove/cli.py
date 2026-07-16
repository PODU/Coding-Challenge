"""Command-line interface for ccbgremove.

Usage examples::

    ccbgremove photo.jpg                 # -> photo-nobg.png
    ccbgremove photo.jpg -o cutout.png   # explicit output name
    ccbgremove photos/ -o out/           # recurse a directory, mirror structure
    ccbgremove a.jpg b.png -o out/       # several files into one output dir
    ccbgremove photo.jpg --backend grabcut --feather 3
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import __version__
from .errors import BgRemoveError
from .pipeline import (
    DEFAULT_SUFFIX,
    Job,
    default_output_path,
    iter_images,
    mirror_output_path,
    process_jobs,
)
from .segment import make_segmenter


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="ccbgremove",
        description="Remove image backgrounds locally, keeping the subject with "
        "a transparent alpha channel.",
    )
    p.add_argument("inputs", nargs="+", type=Path, help="image files or directories")
    p.add_argument(
        "-o", "--output", type=Path, default=None,
        help="output file (single input) or output directory (batch/dirs)",
    )
    p.add_argument(
        "--backend", default="auto",
        choices=["auto", "grabcut", "rembg", "none"],
        help="segmentation backend (default: auto — neural if installed, "
        "else grabcut)",
    )
    p.add_argument(
        "--margin", type=float, default=0.08,
        help="grabcut: border fraction assumed to be background (default 0.08)",
    )
    p.add_argument(
        "--iterations", type=int, default=5,
        help="grabcut: refinement iterations (default 5)",
    )
    p.add_argument(
        "--feather", type=float, default=2.0,
        help="edge softening radius in pixels for smooth hair edges (default 2)",
    )
    p.add_argument(
        "--hard-edges", action="store_true",
        help="disable edge feathering; produce a crisp 0/255 alpha cut",
    )
    p.add_argument(
        "--suffix", default=DEFAULT_SUFFIX,
        help=f"output filename suffix (default '{DEFAULT_SUFFIX}')",
    )
    p.add_argument(
        "--no-recursive", dest="recursive", action="store_false",
        help="do not descend into subdirectories",
    )
    p.add_argument(
        "--overwrite", action="store_true",
        help="allow writing over an existing/identical input path",
    )
    p.add_argument("-q", "--quiet", action="store_true", help="suppress per-file output")
    p.add_argument("--version", action="version", version=f"ccbgremove {__version__}")
    return p


def _plan_jobs(args) -> list[Job]:
    files = [p for p in args.inputs if p.is_file()]
    dirs = [p for p in args.inputs if p.is_dir()]
    missing = [p for p in args.inputs if not p.exists()]
    if missing:
        raise BgRemoveError(
            "input path(s) not found: " + ", ".join(str(m) for m in missing)
        )

    out = args.output
    single_file_mode = len(files) == 1 and not dirs
    # -o is an output directory when it names an existing dir, or when the run
    # produces more than one output (multiple files, or any directory input).
    out_is_dir = out is not None and (
        out.is_dir() or not single_file_mode
    )

    jobs: list[Job] = []

    for f in files:
        if single_file_mode and out is not None and not out_is_dir:
            dst = out
        else:
            out_dir = out if out_is_dir else None
            dst = default_output_path(f, out_dir=out_dir, suffix=args.suffix)
        jobs.append(Job(src=f, dst=dst))

    for d in dirs:
        # Mirror the directory tree so files with the same name don't collide.
        out_root = out if out is not None else d.parent / f"{d.name}{args.suffix}"
        for img in iter_images(d, recursive=args.recursive):
            dst = mirror_output_path(img, d, out_root, suffix=args.suffix)
            jobs.append(Job(src=img, dst=dst))

    return jobs


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)

    try:
        segmenter = make_segmenter(
            args.backend, margin=args.margin, iterations=args.iterations
        )
    except (ValueError, RuntimeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    try:
        jobs = _plan_jobs(args)
    except BgRemoveError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    if not jobs:
        print("error: no JPG/PNG images found to process", file=sys.stderr)
        return 1

    def on_event(kind: str, src: Path, detail: str) -> None:
        if kind == "fail":
            print(f"FAIL  {src}: {detail}", file=sys.stderr)
        elif not args.quiet:
            print(f"ok    {src} -> {detail}")

    result = process_jobs(
        jobs, segmenter,
        feather=args.feather, hard=args.hard_edges, overwrite=args.overwrite,
        on_event=on_event,
    )

    failed = len(result.failures)
    summary = f"{len(result.successes)}/{len(jobs)} succeeded"
    if failed:
        print(f"{summary}, {failed} failed:", file=sys.stderr)
        for path, err in result.failures:
            print(f"  - {path}: {err}", file=sys.stderr)
        return 1

    if not args.quiet:
        print(summary)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
