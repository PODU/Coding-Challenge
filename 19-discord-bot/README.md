# disdaccbot — Coding Challenge #19 (Discord bot)

A Discord bot that connects to the gateway, greets users, serves random quotes
and coding challenges, and lets users grow the challenge catalog by submitting
validated `codingchallenges.fyi` URLs. Built in TypeScript with
[discord.js](https://discord.js.org) v14.

Command logic is fully decoupled from the Discord transport so every handler is
unit-tested with no live gateway connection and no real network.

## Features

| Trigger | Slash equivalent | Behaviour |
| --- | --- | --- |
| `Hello` (anywhere in a message) | — | Replies `Hello, <username>!` |
| `!quote` | `/quote` | Random quote from the dummyJSON API |
| `!challenge` | `/challenge` | Random challenge from the local catalog |
| `!add <url>` | `/add url:<url>` | Validates the host is `codingchallenges.fyi`, scrapes the `<title>`, and persists it (deduped) |
| `!list` | `/list` | Lists all challenges (truncated under Discord's 2000-char limit) |

`!` is the default prefix (configurable via `COMMAND_PREFIX`).

## Requirements

- Node.js >= 18 (uses the built-in global `fetch`)
- A Discord application + bot token (see below)

## Setup

```bash
cd 19-discord-bot
npm install
cp .env.example .env   # then edit .env
```

### Create the bot in the Discord Developer Portal

1. https://discord.com/developers/applications → **New Application**.
2. **Bot** tab → **Reset Token** → copy it into `DISCORD_TOKEN` in `.env`.
3. **Bot** tab → enable the **MESSAGE CONTENT INTENT** (privileged). This is
   required to read the text of `!` commands and the `Hello` greeting.
4. **General Information** → copy the **Application ID** into `DISCORD_APP_ID`.
5. **OAuth2 → URL Generator**: scopes `bot` + `applications.commands`,
   bot permissions `Send Messages` + `Read Message History`. Open the generated
   URL to invite the bot to your server.

### Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DISCORD_TOKEN` | yes | — | Bot token. Never hardcoded or committed. |
| `DISCORD_APP_ID` | for slash registration | — | Application (client) id. |
| `DISCORD_GUILD_ID` | no | — | Register slash commands to one guild (instant) for dev; omit for global. |
| `COMMAND_PREFIX` | no | `!` | Text-command prefix. |
| `QUOTES_API_URL` | no | `https://dummyjson.com/quotes/random` | Quote endpoint. |
| `CATALOG_PATH` | no | `./data/challenges.json` | Persisted catalog file. |
| `LOG_LEVEL` | no | `info` | `error` \| `warn` \| `info` \| `debug`. |

The token is read only by `src/config.ts`; no other module touches
`process.env`. Startup fails fast with a clear message if `DISCORD_TOKEN` is
missing.

## Register slash commands

Slash commands must be registered once (and again whenever they change):

```bash
# dev: set DISCORD_GUILD_ID in .env for instant, per-guild registration
npm run register

# or after building:
npm run build
npm run register:built
```

With `DISCORD_GUILD_ID` unset the commands register globally (can take up to an
hour to propagate) — use that for production.

## Run

```bash
# development (ts-node, no build step)
npm run dev

# production
npm run build      # tsc -> dist/
npm start          # node dist/index.js
```

On first run the catalog is seeded from `seed/challenges.seed.json` into
`CATALOG_PATH`; added challenges persist there across restarts.

## Test

The suite is fully offline — `fetch` is mocked, no Discord connection, no real
network — so it is deterministic in CI.

```bash
npm test           # vitest run
npm run typecheck  # tsc --noEmit
```

Covered: every command handler (with stubbed services), the prefix
router/parser + greeting match, the slash adapter, the `quotes` service against
a mocked fetch (200 / 500 / malformed), the `http` helpers (`extractTitle`,
`isCodingChallengesUrl` incl. spoofed subdomains), the `catalog` service
(seeding, round-trip persistence, dedupe, corrupt-file recovery, concurrent
writes), and config validation.

## Production deployment

- **systemd** (Linux/EC2): `deploy/disdaccbot.service` — `Restart=always`,
  start-on-boot, token supplied via `EnvironmentFile=/etc/disdaccbot.env`
  (chmod 600). Build with `npm ci && npm run build`, deploy `dist/` + `seed/` +
  `node_modules`, then `systemctl enable --now disdaccbot`.
- **Docker**: `deploy/Dockerfile` is a multi-stage build (`node:20-slim`
  runtime). Pass the token at runtime: `docker run --env-file .env <image>`.
  Mount a volume at `/app/data` to persist the catalog.

Logs are single-line JSON on stdout/stderr (captured by journald/Docker). The
process installs `SIGINT`/`SIGTERM` graceful shutdown and logs (rather than
dies on) unhandled rejections and gateway disconnects.

## Architecture

```
Discord Gateway (WSS)
  -> src/client.ts   (discord.js Client; intents: Guilds, GuildMessages, MessageContent)
       messageCreate     -> src/router.ts  (prefix parse + "Hello")
       interactionCreate -> src/slash.ts   (app-command adapter)
            both build a CommandContext { args, user, reply, deps }
              -> src/commands/*.ts  (hello, quote, challenge, add, list)
                   -> src/services/*.ts (quotes, catalog, http)
                        -> data/challenges.json
```

The `CommandContext` abstraction (`src/context.ts`) is the seam that keeps
handlers free of discord.js types and therefore unit-testable.
