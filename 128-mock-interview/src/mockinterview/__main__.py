"""CLI entry point: `mockinterview` or `python -m mockinterview`."""

from __future__ import annotations

import argparse

import uvicorn

from .app import create_app
from .config import get_settings


def main() -> None:
    parser = argparse.ArgumentParser(description="Mock Interview Coach server")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8000)
    args = parser.parse_args()

    settings = get_settings()
    print(f"Anam video:       {'LIVE' if settings.anam_configured else 'MOCK (text mode, set ANAM_API_KEY for video)'}")
    print(f"Claude feedback:  {'LIVE' if settings.anthropic_configured else 'MOCK (heuristic, set ANTHROPIC_API_KEY for real analysis)'}")
    print(f"Open http://{args.host}:{args.port}")

    uvicorn.run(create_app(settings), host=args.host, port=args.port)


if __name__ == "__main__":
    main()
