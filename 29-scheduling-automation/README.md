# Scheduling Automation — Coding Challenge #29

A simplified Calendly/Cal.com-style scheduler. It connects to one or more
calendars over **CalDAV** (with app-password or OAuth 2.0 auth), reads existing
events to learn when you are busy, lets you declare a recurring **weekly
availability**, computes the resulting **free slots** for a requested meeting
duration, and lets an invitee **book** a slot — writing a new event back to the
calendar and inviting the attendee.

The heart of the project is a **pure, deterministic free-slot engine**
(`scheduler.domain`) with no I/O and no wall-clock reads, exhaustively unit
tested. All CalDAV/OAuth/network access sits behind a `CalendarProvider`
interface and is mocked in tests, so the suite runs fully offline.

## Architecture

```
Interface layer    cli.py · api.py (FastAPI)
        │
Service layer      SchedulingService  (orchestration; injected Clock + Provider)
        │
   ┌────┴─────────────────────────┐
Domain (pure)                 Provider / transport
  interval.py merge/subtract    base.py        CalendarProvider protocol + Event
  availability.py expand        caldav_provider.py  real CalDAV + ICS/RRULE
  slots.py    free_slots()      ics.py         iCalendar parse + recurrence
  clock.py    Clock protocol    auth.py        OAuth2 refresh / app-password
                                fake_provider.py    in-memory fake for tests
```

Key invariants:

- **Everything is UTC internally.** Providers normalise every event to UTC; the
  organizer timezone is applied at the edges (availability expansion, all-day
  events, presentation).
- **The domain core is pure.** `free_slots(availability, busy, req, now)` takes
  data in and returns data out — `now` is injected, never read from the clock.
- **Booking is race-safe.** The service re-validates the slot against a fresh
  busy fetch and the provider performs a conditional create; a losing racer
  gets `409`.

## Setup

Requires Python 3.12+.

```bash
# from the 29-scheduling-automation directory
python -m pip install -e ".[dev]"     # installs the package + dev tools
# or just the runtime deps:
python -m pip install -e .
```

Copy the example environment file and fill in your credentials:

```bash
cp .env.example .env
```

### Environment variables

| Variable | Purpose |
|---|---|
| `CALDAV_URL` | CalDAV base URL |
| `CALDAV_USERNAME` / `CALDAV_APP_PASSWORD` | Basic-auth credentials (app password) |
| `OAUTH_CLIENT_ID` / `OAUTH_CLIENT_SECRET` / `OAUTH_TOKEN_JSON` | OAuth 2.0 alternative (token bundle as JSON) |
| `CALENDARS` | Comma-separated calendar names to consolidate (empty = all) |
| `ORGANIZER_TZ` | IANA timezone, e.g. `Europe/London` |
| `ORGANIZER_EMAIL` | Organizer email used as `ORGANIZER` on bookings |

Secrets are read from the environment only — nothing is hard-coded.

## Running

### CLI

The package installs a `scheduler` console script (also runnable as
`python -m scheduler.cli`):

```bash
scheduler events --from 2026-07-01 --to 2026-07-02
scheduler slots  --duration 30m --from 2026-07-01 --to 2026-07-07 --step 15m
scheduler book   --start 2026-07-02T14:00Z --duration 30m \
                 --attendee "Sam <sam@x.com>" --title "Intro call"
```

Duration syntax: `30m`, `1h`, `1h30m`. Exit codes: `0` ok, `2` bad args,
`3` slot no longer available, `4` provider/auth error.

### HTTP API

```bash
python -c "from scheduler.factory import build_service; \
           from scheduler.api import create_app; import uvicorn; \
           uvicorn.run(create_app(build_service()), host='0.0.0.0', port=8000)"
```

| Method & path | Purpose |
|---|---|
| `GET /healthz` | Liveness |
| `GET /availability` | Configured weekly availability |
| `GET /slots?duration=30&from=...&to=...&step=15` | Free slots (minutes) |
| `POST /book` | Book a slot — `201 {uid}` or `409` if taken |

## Testing

The suite is deterministic and offline — every clock is a `FixedClock` and all
calendar I/O goes through `FakeProvider` seeded from `.ics` fixtures.

```bash
python -m pytest
```

Coverage spans the pure interval/slot math (overlap, merge, subtract,
back-to-back, buffers, min-notice, step variations, duration filtering),
timezone/DST and midnight-crossing availability, RRULE/EXDATE recurrence and
all-day expansion, multi-calendar consolidation/dedup, OAuth token refresh, and
the service/API/CLI booking paths (success, race `409`, stale-slot re-validation,
validation rejections).

## Design

