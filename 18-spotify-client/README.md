# Spotify Client (Coding Challenge #18)

A production-ready, command-line Spotify client. It authenticates with the
Spotify Web API using **OAuth 2.0 Authorization Code with PKCE** (no client
secret ever stored), then lets you browse playlists, search the catalogue, list
playback devices, transfer playback, and control play/pause/next/previous on any
device already running an active Spotify session.

This is the **backend CLI track** of the challenge: it remote-controls an
existing Spotify device (phone, desktop app, speaker) rather than turning a
browser tab into a player.

## Features

- Authorization Code + PKCE login via the system browser and a throwaway
  loopback callback server; verified by `GET /v1/me`.
- List playlists (title + artwork URL), with pagination.
- Search tracks.
- List devices and **transfer playback** to a chosen device by id or name.
- Control playback: `play` (optionally a playlist), `pause`, `next`, `prev`,
  `now`.
- Transparent, automatic **token refresh** (plus a single refresh-and-retry on
  `401`).
- Graceful handling of **no active device**, Premium-required, and `429`
  rate-limit responses.
- Secrets via **environment variables only** — never hardcoded or committed.
- Tokens cached in the OS config dir with `0600` permissions; never logged.

## Requirements

- Python **3.11+**
- A Spotify account (Premium is required for *remote playback control*; login,
  search, and playlist listing work on Free).

## Setup

### 1. Register an app on the Spotify Developer Dashboard

1. Go to <https://developer.spotify.com/dashboard> and create an app.
2. Copy the **Client ID**.
3. Add a **Redirect URI**: `http://127.0.0.1:8888/callback` (must match exactly).
   PKCE means you do **not** need the client secret.

### 2. Configure environment variables

Copy the example file and fill in your Client ID:

```bash
cp .env.example .env
# edit .env
```

| Variable | Required | Default | Notes |
|---|---|---|---|
| `SPOTIFY_CLIENT_ID` | yes | — | Client ID from the dashboard. |
| `SPOTIFY_REDIRECT_URI` | no | `http://127.0.0.1:8888/callback` | Must match a registered redirect URI. |

The `.env` file is loaded automatically **only if** `python-dotenv` is installed
(`pip install -e ".[dotenv]"`). Otherwise export the variables in your shell:

```bash
export SPOTIFY_CLIENT_ID=your-client-id
```

### 3. Install

```bash
python -m pip install -e .
# optional extras:
python -m pip install -e ".[dotenv,rich]"
```

This installs a `spotify` console script. You can also run it as a module:
`python -m spotify_client`.

## Usage

```bash
spotify login                       # browser auth, verifies via /v1/me
spotify me                          # show your profile
spotify playlists [--limit N]       # list playlists (title + artwork URL)
spotify search "daft punk" [--limit N]
spotify devices                     # list available devices
spotify now                         # currently playing track
spotify play [--playlist <uri|index>] [--device <id|name>]
spotify pause [--device <id|name>]
spotify next [--device <id|name>]
spotify prev [--device <id|name>]
spotify transfer <device id|name> [--no-play]
spotify logout                      # clear the cached token
```

Add the global `--json` flag (before the subcommand) for machine-readable
output, e.g. `spotify --json playlists`.

`--playlist` accepts either a `spotify:playlist:...` URI or an integer index
from the `spotify playlists` listing.

### Example

```bash
spotify login
spotify devices
# - Living Room Speaker  type=Speaker  id=abc123  vol=40
spotify transfer "Living Room Speaker"
spotify play --playlist 0
spotify now
```

## Configuration & secrets

- No secret is ever stored: PKCE removes the client secret entirely.
- `SPOTIFY_CLIENT_ID` / `SPOTIFY_REDIRECT_URI` come from the environment (or a
  git-ignored `.env`).
- Tokens are cached at:
  - Windows: `%APPDATA%\spotify-client\tokens.json`
  - Linux/macOS: `$XDG_CONFIG_HOME/spotify-client/tokens.json` (or `~/.config/...`)
  - written with `0600` permissions and never logged.

## Scopes requested

```
user-read-private user-read-email playlist-read-private
user-read-playback-state user-modify-playback-state user-read-currently-playing
```

## Testing

The suite is **fully offline** — all HTTP is mocked with
[`responses`](https://github.com/getsentry/responses); no real Spotify
credentials or network access are required.

```bash
python -m pip install -e ".[test]"
python -m pytest
```

With coverage:

```bash
python -m pytest --cov=spotify_client --cov-report=term-missing
```

The tests cover PKCE challenge/URL generation, token exchange and transparent
refresh (including refresh failure), the no-active-device path, Premium and
rate-limit mapping, single refresh-and-retry on `401`, search, paginated
playlist listing, and CLI command dispatch / exit codes.

## Project layout

```
18-spotify-client/
├── pyproject.toml          # package metadata + console script `spotify`
├── .env.example            # documents env vars (no secret)
├── src/spotify_client/
│   ├── cli.py              # argparse command dispatch
│   ├── config.py           # env-var loading, scopes, endpoints
│   ├── auth.py             # PKCE flow, callback server, token refresh
│   ├── tokenstore.py       # 0600 token persistence
│   ├── api.py              # SpotifyClient: typed Web API wrapper
│   ├── models.py           # Track / Playlist / Device / Playback dataclasses
│   └── errors.py           # typed error hierarchy
└── tests/                  # offline pytest suite (responses-mocked)
```

## License

MIT.
