"""Command-line interface: ``spotify <command>`` via argparse."""

from __future__ import annotations

import argparse
import json as jsonlib
import sys
from typing import List, Optional, Sequence

from .api import SpotifyClient
from .auth import AuthManager
from .config import Config
from .errors import (
    AuthError,
    ConfigError,
    NoActiveDeviceError,
    PremiumRequiredError,
    RateLimited,
    SpotifyError,
)
from .models import Device, Playback, Playlist, Track
from .tokenstore import TokenStore

EXIT_OK = 0
EXIT_ERROR = 1
EXIT_AUTH = 2
EXIT_NO_DEVICE = 3


def build_client(config: Optional[Config] = None) -> SpotifyClient:
    """Tests monkeypatch this to swap in a fake client."""
    return SpotifyClient(AuthManager(config or Config.from_env(), TokenStore()))


def _emit(obj: object, as_json: bool, text: str) -> None:
    if as_json:
        print(jsonlib.dumps(obj, indent=2, default=_json_default))
    else:
        print(text)


def _json_default(o: object) -> object:
    if hasattr(o, "__dict__"):
        return o.__dict__
    return str(o)


def cmd_login(client: SpotifyClient, args: argparse.Namespace) -> int:
    client.auth.login()
    profile = client.current_user()
    _emit(profile, args.json,
          f"Logged in as {profile.get('display_name') or profile.get('id')} "
          f"(id: {profile.get('id')}).")
    return EXIT_OK


def cmd_me(client: SpotifyClient, args: argparse.Namespace) -> int:
    profile = client.current_user()
    _emit(profile, args.json,
          f"{profile.get('display_name') or profile.get('id')} "
          f"(id: {profile.get('id')}, product: {profile.get('product', 'unknown')})")
    return EXIT_OK


def cmd_playlists(client: SpotifyClient, args: argparse.Namespace) -> int:
    items: List[Playlist] = client.playlists(limit=args.limit)
    if args.json:
        _emit(items, True, "")
        return EXIT_OK
    if not items:
        print("No playlists found.")
        return EXIT_OK
    for i, p in enumerate(items):
        print(f"[{i}] {p.name}  ({p.track_count} tracks)  owner={p.owner}")
        print(f"      uri: {p.uri}")
        print(f"      art: {p.image_url or '-'}")
    return EXIT_OK


def cmd_search(client: SpotifyClient, args: argparse.Namespace) -> int:
    tracks: List[Track] = client.search_tracks(args.query, limit=args.limit)
    if args.json:
        _emit(tracks, True, "")
        return EXIT_OK
    if not tracks:
        print("No tracks found.")
        return EXIT_OK
    for i, t in enumerate(tracks):
        print(f"[{i}] {t.name} - {t.artist_names}  [{t.album}]")
        print(f"      uri: {t.uri}")
    return EXIT_OK


def cmd_devices(client: SpotifyClient, args: argparse.Namespace) -> int:
    devices: List[Device] = client.devices()
    if args.json:
        _emit(devices, True, "")
        return EXIT_OK
    if not devices:
        print("No devices found. Open Spotify on a phone, desktop, or speaker.")
        return EXIT_OK
    for d in devices:
        active = " (active)" if d.is_active else ""
        print(f"- {d.name}{active}  type={d.type}  id={d.id}  vol={d.volume_percent}")
    return EXIT_OK


def cmd_now(client: SpotifyClient, args: argparse.Namespace) -> int:
    playback: Playback = client.current_playback()
    if args.json:
        _emit(playback, True, "")
        return EXIT_OK
    state = "playing" if playback.is_playing else "paused"
    dev = playback.device.name if playback.device else "unknown device"
    if playback.track:
        t = playback.track
        print(f"{state} on {dev}: {t.name} - {t.artist_names}  [{t.album}]")
    else:
        print(f"{state} on {dev}: (nothing)")
    return EXIT_OK


def _resolve_device_id(client: SpotifyClient,
                       selector: Optional[str]) -> Optional[str]:
    if not selector:
        return None
    return client.resolve_device(selector).id


def cmd_play(client: SpotifyClient, args: argparse.Namespace) -> int:
    device_id = _resolve_device_id(client, args.device)
    context_uri = None
    if args.playlist is not None:
        context_uri = _resolve_playlist_uri(client, args.playlist)
    client.play(context_uri=context_uri, device_id=device_id)
    print("Playback started." if context_uri else "Playback resumed.")
    return EXIT_OK


def _resolve_playlist_uri(client: SpotifyClient, selector: str) -> str:
    """Accept a URI as-is, or a number from ``spotify playlists``."""
    if selector.startswith("spotify:") or selector.startswith("http"):
        return selector
    if selector.isdigit():
        items = client.playlists()
        idx = int(selector)
        if 0 <= idx < len(items) and items[idx].uri:
            return items[idx].uri  # type: ignore[return-value]
        raise SpotifyError(f"Playlist index {idx} out of range.")
    raise SpotifyError(
        f"Could not resolve playlist '{selector}'. Pass a spotify: URI or an "
        "index from `spotify playlists`."
    )


def cmd_pause(client: SpotifyClient, args: argparse.Namespace) -> int:
    client.pause(device_id=_resolve_device_id(client, args.device))
    print("Paused.")
    return EXIT_OK


def cmd_next(client: SpotifyClient, args: argparse.Namespace) -> int:
    client.next(device_id=_resolve_device_id(client, args.device))
    print("Skipped to next track.")
    return EXIT_OK


def cmd_prev(client: SpotifyClient, args: argparse.Namespace) -> int:
    client.previous(device_id=_resolve_device_id(client, args.device))
    print("Skipped to previous track.")
    return EXIT_OK


def cmd_transfer(client: SpotifyClient, args: argparse.Namespace) -> int:
    device = client.resolve_device(args.device)
    client.transfer(device.id, play=not args.no_play)  # type: ignore[arg-type]
    print(f"Transferred playback to {device.name}.")
    return EXIT_OK


def cmd_logout(client: SpotifyClient, args: argparse.Namespace) -> int:
    client.auth.store.clear()
    print("Logged out (token cache cleared).")
    return EXIT_OK


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="spotify",
        description="Command-line Spotify client (OAuth PKCE).",
    )
    parser.add_argument("--json", action="store_true",
                        help="Emit machine-readable JSON output.")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("login", help="Authenticate via browser (PKCE).").set_defaults(func=cmd_login)
    sub.add_parser("logout", help="Clear the stored token cache.").set_defaults(func=cmd_logout)
    sub.add_parser("me", help="Show the current user profile.").set_defaults(func=cmd_me)

    p_pl = sub.add_parser("playlists", help="List your playlists.")
    p_pl.add_argument("--limit", type=int, default=50)
    p_pl.set_defaults(func=cmd_playlists)

    p_search = sub.add_parser("search", help="Search for tracks.")
    p_search.add_argument("query")
    p_search.add_argument("--limit", type=int, default=20)
    p_search.set_defaults(func=cmd_search)

    sub.add_parser("devices", help="List available devices.").set_defaults(func=cmd_devices)
    sub.add_parser("now", help="Show the currently playing track.").set_defaults(func=cmd_now)

    p_play = sub.add_parser("play", help="Play/resume (optionally a playlist).")
    p_play.add_argument("--playlist", help="spotify: URI or index from `playlists`.")
    p_play.add_argument("--device", help="Target device id or name.")
    p_play.set_defaults(func=cmd_play)

    p_pause = sub.add_parser("pause", help="Pause playback.")
    p_pause.add_argument("--device", help="Target device id or name.")
    p_pause.set_defaults(func=cmd_pause)

    p_next = sub.add_parser("next", help="Next track.")
    p_next.add_argument("--device", help="Target device id or name.")
    p_next.set_defaults(func=cmd_next)

    p_prev = sub.add_parser("prev", help="Previous track.")
    p_prev.add_argument("--device", help="Target device id or name.")
    p_prev.set_defaults(func=cmd_prev)

    p_tr = sub.add_parser("transfer", help="Transfer playback to a device.")
    p_tr.add_argument("device", help="Target device id or name.")
    p_tr.add_argument("--no-play", action="store_true",
                      help="Transfer without auto-starting playback.")
    p_tr.set_defaults(func=cmd_transfer)

    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = build_parser().parse_args(argv)

    try:
        client = build_client()
    except ConfigError as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        return EXIT_ERROR

    try:
        return args.func(client, args)
    except NoActiveDeviceError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_NO_DEVICE
    except AuthError as exc:
        print(f"Authentication error: {exc}", file=sys.stderr)
        return EXIT_AUTH
    except PremiumRequiredError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_ERROR
    except RateLimited as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_ERROR
    except SpotifyError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return EXIT_ERROR


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
