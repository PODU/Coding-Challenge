# Spotify Playlist Manager — Coding Challenge #64

A production-ready Python CLI to **back up**, **restore**, and **manage** Spotify
playlists via the [Spotify Web API](https://developer.spotify.com/documentation/web-api).

- Back up any **public** playlist (Client Credentials flow) or **all your own /
  private** playlists (Authorization Code + PKCE flow) to **CSV** (or zipped CSV).
- **Restore / create** a playlist from a backup CSV: searches Spotify for each
  track, **dedupes by URI**, and adds tracks in **100-item batches**.
- Transparent **token refresh** (on 401 and proactive expiry) and **rate-limit
  backoff** (honors `429` `Retry-After`).
- Credentials live in **environment variables / `.env`** — never hardcoded.
- **Fully offline test suite** — every HTTP call is mocked; no real credentials.

>

---

## Install

```bash
python -m pip install -e ".[dev]"   # dev extras = pytest + responses
```

This installs a console script: `spotify-playlist`.

## Configure (no secrets in code)

1. Register an app at the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   to get a **Client ID** / **Client Secret**, and add a Redirect URI
   (e.g. `http://127.0.0.1:8888/callback`) for user login.
2. Copy `.env.example` to `.env` and fill it in (`.env` is git-ignored):

| Variable | Purpose |
|----------|---------|
| `SPOTIFY_CLIENT_ID` | App client id (required) |
| `SPOTIFY_CLIENT_SECRET` | App client secret (required) |
| `SPOTIFY_REDIRECT_URI` | Redirect URI for the user flow (`list`/`restore`/`backup --user`) |
| `SPOTIFY_TOKEN_CACHE` | Refresh-token cache path (default `~/.spotify_playlist_token.json`, 0600) |

The CLI auto-loads a `.env` from the working directory if present.

## Usage

```bash
# Back up a PUBLIC playlist (no login) -> CSV
spotify-playlist backup 37i9dQZF1DXcBWIGoYBM5M -o backup.csv

# Accepts raw id, spotify:playlist:ID URI, or an open.spotify.com URL
spotify-playlist backup https://open.spotify.com/playlist/37i9dQ... --zip

# Back up one of YOUR playlists (incl. private) — opens a browser to log in once
spotify-playlist backup <playlist_id> --user

# List all of your playlists (user login)
spotify-playlist list

# Restore / create a new playlist from a backup CSV (user login)
spotify-playlist restore backup.csv --name "My Restored Playlist" --public
```

The user (Authorization Code) flow opens a browser, runs a short-lived loopback
server on your redirect URI to capture the auth code (PKCE), then caches a
refresh token (0600) so subsequent runs are non-interactive.

### CSV format

Header: `track_name,artists,album,track_uri,duration_ms,added_at,isrc`
(artists are `; `-separated). Standard CSV quoting handles commas, quotes and
unicode. This neutral format makes backups portable and migration-friendly.

## How it works

| Module | Responsibility |
|--------|----------------|
| `config.py` | Loads/validates credentials from env (`.env` optional). |
| `auth.py` | OAuth: Client Credentials, Authorization Code + PKCE, refresh, 0600 token cache. |
| `client.py` | `requests` wrapper: bearer injection, pagination (`next`), 401→refresh, 429 backoff. |
| `models.py` | `Track` / `Playlist` dataclasses. |
| `playlist.py` | Backup (normalize, skip null/local/episode), restore (search→dedupe→batched add). |
| `exporter.py` | CSV / zipped-CSV write + read (round-trippable). |
| `cli.py` | `argparse` sub-commands `list` / `backup` / `restore`. |

## Tests — fully offline, no credentials

```bash
python -m pytest
```

All HTTP is stubbed with [`responses`](https://github.com/getsentry/responses);
dummy credentials are injected via fixtures. Coverage: pagination, CSV
export/round-trip, dedupe, 100-item chunking, 429 backoff, 401 refresh, and
auth-URL / PKCE generation.

## License

MIT
