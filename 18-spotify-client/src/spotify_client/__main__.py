"""Allow ``python -m spotify_client`` to invoke the CLI."""

from __future__ import annotations

from .cli import main

if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
