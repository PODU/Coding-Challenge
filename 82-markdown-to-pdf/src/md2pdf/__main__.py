"""Lets ``python -m md2pdf`` work like the md2pdf command."""

from __future__ import annotations

from .cli import main

if __name__ == "__main__":
    raise SystemExit(main())
