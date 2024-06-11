"""Command-line interface: ``backup`` | ``restore`` | ``list``."""

from __future__ import annotations

import argparse
import logging
import sys
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import List, Optional, Sequence
from urllib.parse import parse_qs, urlparse

from . import __version__
from .auth import (
    AuthorizationCodeTokenProvider,
    ClientCredentialsTokenProvider,
    TokenProvider,
)
from .client import SpotifyClient
from .config import Config, ConfigError, load_dotenv
from .exporter import read_csv, write_csv, write_zip
from .playlist import (
    backup_playlist,
    list_my_playlists,
    restore_playlist,
)

log = logging.getLogger(__name__)


def loopback_authorizer(consent_url: str) -> str:  # pragma: no cover - interactive
    """Open the consent page in a browser and catch the redirect on a local
    server to get the ``code``. If the port can't be bound, ask the user to
    paste the redirect URL instead.
    """
    parsed = urlparse(consent_url)
    redirect = parse_qs(parsed.query).get("redirect_uri", [""])[0]
    redirect_parsed = urlparse(redirect)
    host = redirect_parsed.hostname or "127.0.0.1"
    port = redirect_parsed.port or 8888

    print("Opening browser for Spotify login...")
    print(f"If it does not open, visit:\n{consent_url}\n")
    webbrowser.open(consent_url)

    captured: dict[str, str] = {}

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:  # noqa: N802
            qs = parse_qs(urlparse(self.path).query)
            captured.update({k: v[0] for k, v in qs.items()})
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(b"<h1>Authentication complete. You may close this tab.</h1>")

        def log_message(self, *args: object) -> None:
            pass

    try:
        server = HTTPServer((host, port), Handler)
    except OSError:
        return input("Paste the full redirect URL (or the code): ").strip().split("code=")[-1]

    try:
        server.handle_request()
    finally:
        server.server_close()

    if "error" in captured:
        raise RuntimeError(f"Authorization denied: {captured['error']}")
    return captured.get("code", "")


def _make_provider(config: Config, *, user: bool) -> TokenProvider:
    if user:
        return AuthorizationCodeTokenProvider(config, authorizer=loopback_authorizer)
    return ClientCredentialsTokenProvider(config)


def _make_client(config: Config, *, user: bool) -> SpotifyClient:
    return SpotifyClient(_make_provider(config, user=user))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="spotify-playlist",
        description="Backup, restore and manage Spotify playlists.",
    )
    parser.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    parser.add_argument(
        "-v", "--verbose", action="store_true", help="Enable debug logging."
    )
    sub = parser.add_subparsers(dest="command", required=True)

    p_list = sub.add_parser("list", help="List your playlists (requires user login).")
    p_list.set_defaults(func=cmd_list, user=True)

    p_backup = sub.add_parser("backup", help="Export a playlist to CSV.")
    p_backup.add_argument("playlist_id", help="Spotify playlist id or URI.")
    p_backup.add_argument("-o", "--output", help="Output path (default <name>.csv).")
    p_backup.add_argument("--zip", action="store_true", help="Write a zipped CSV.")
    p_backup.add_argument(
        "--user", action="store_true", help="Use user login (for private playlists)."
    )
    p_backup.set_defaults(func=cmd_backup)

    p_restore = sub.add_parser("restore", help="Create a playlist from a backup CSV.")
    p_restore.add_argument("input", help="Backup CSV (or zip) to restore.")
    p_restore.add_argument("--name", required=True, help="Name for the new playlist.")
    p_restore.add_argument("--public", action="store_true", help="Make the playlist public.")
    p_restore.set_defaults(func=cmd_restore, user=True)

    return parser


def _playlist_id(value: str) -> str:
    """Accept a raw id, a ``spotify:playlist:ID`` URI, or an open.spotify URL."""
    if "spotify:playlist:" in value:
        return value.rsplit(":", 1)[-1]
    if "open.spotify.com" in value:
        return urlparse(value).path.rstrip("/").rsplit("/", 1)[-1].split("?")[0]
    return value


def cmd_list(args: argparse.Namespace, config: Config) -> int:
    client = _make_client(config, user=True)
    playlists = list_my_playlists(client)
    if not playlists:
        print("No playlists found.")
        return 0
    for pl in playlists:
        total = (pl.get("tracks") or {}).get("total", "?")
        print(f"{pl.get('id', ''):24}  {total:>5}  {pl.get('name', '')}")
    return 0


def cmd_backup(args: argparse.Namespace, config: Config) -> int:
    client = _make_client(config, user=bool(getattr(args, "user", False)))
    playlist = backup_playlist(client, _playlist_id(args.playlist_id))

    default_name = (playlist.name or playlist.id or "playlist").replace("/", "_")
    if args.output:
        out = args.output
    else:
        out = f"{default_name}.zip" if args.zip else f"{default_name}.csv"

    if args.zip:
        write_zip(playlist, out)
    else:
        write_csv(playlist, out)
    print(f"Backed up {len(playlist)} tracks from {playlist.name!r} -> {out}")
    return 0


def cmd_restore(args: argparse.Namespace, config: Config) -> int:
    client = _make_client(config, user=True)
    rows = read_csv(args.input)
    result = restore_playlist(client, rows, name=args.name, public=args.public)
    print(
        f"Restored {result.added} tracks into playlist {result.playlist_id} "
        f"({result.duplicates} duplicates skipped, {len(result.misses)} not found)."
    )
    if result.playlist_url:
        print(f"Open: {result.playlist_url}")
    if result.misses:
        print("Could not restore:")
        for miss in result.misses:
            print(f"  - {miss.get('track_name', '')} / {miss.get('artists', '')}")
    return 0


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    logging.basicConfig(
        level=logging.DEBUG if getattr(args, "verbose", False) else logging.INFO,
        format="%(levelname)s %(name)s: %(message)s",
    )

    try:
        load_dotenv()
        config = Config.from_env(require_redirect=bool(getattr(args, "user", False)))
        return args.func(args, config)
    except ConfigError as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        return 2
    except Exception as exc:  # pragma: no cover - top-level guard
        print(f"Error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
