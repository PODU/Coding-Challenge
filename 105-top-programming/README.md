# Coding Challenge #105 — Top Programming Stories Dashboard

A streaming-aggregation microservice pipeline that consolidates the top
programming / software-engineering / AI stories from **Hacker News** (Firebase
API) into a single filterable dashboard.

```
Hacker News Firebase API ─▶ Scraper/Producer ─▶ broker topic "hn-stories"
                                                         │
                                                         ▼
            Web Frontend ◀─ REST API ◀─ Consumer ─▶ in-memory store
```

Every network and broker boundary is a Go **interface** with a real
implementation and an offline fake, so the aggregation / ranking / filtering
core and the REST API are tested **fully offline** — no Hacker News network
calls, no Kafka broker.

## Kafka is optional

Kafka is **not required** to build, test, or run the dashboard:

- **Tests** use an in-process channel bus (`internal/broker/memory`) and a
  fixture-backed HN source. `go test ./...` and `go test -race ./...` pass with
  no network and no broker.
- **Local run**: `cmd/api` defaults to `-broker=memory` and runs an in-process
  scraper, so the dashboard works against live Hacker News with no Kafka.
- **Production run**: pass `-broker=kafka` to `cmd/producer` and `cmd/api` to use
  the real `segmentio/kafka-go` implementation (`internal/broker/kafka`).

## Layout

```
cmd/producer/   scrapes HN, publishes to the broker on a poll loop
cmd/api/        consumes into an in-memory store, serves REST + web frontend
internal/
  model/        normalized Story type + Validate()
  source/       Source interface (network boundary) + hackernews client + fixtures
  pipeline/     concurrent fan-out aggregator, dedup, producer publish helper
  ranking/      pure filtering (score/time/type/keyword) + TopN ranking
  broker/       Producer/Consumer interfaces + memory + kafka implementations
  store/        thread-safe in-memory store
  httpapi/      REST handlers
  config/       config.yaml loader + env overrides
web/            auto-refreshing frontend (embedded into the api binary)
testdata/       config fixture
```

## Run (offline / no Kafka)

```sh
go run ./cmd/api            # serves http://localhost:8080, scrapes HN in-process
```

Open <http://localhost:8080>. The page auto-refreshes every 15s and supports
keyword / min-score / type filters.

## Run with Kafka (production path)

```sh
# 1. point at your cluster (Aiven free tier, local broker, ...)
export KAFKA_BROKERS=localhost:9092
export KAFKA_TOPIC=hn-stories

# 2. producer: scrape HN -> Kafka
go run ./cmd/producer -broker=kafka

# 3. api: Kafka -> store -> REST + web
go run ./cmd/api -broker=kafka
```

## REST API

| Method | Path            | Query params                                                  | Returns              |
|--------|-----------------|--------------------------------------------------------------|----------------------|
| GET    | `/stories`      | `limit`, `offset`, `min_score`, `since`, `until`, `type`, `q`| paginated `[]Story`  |
| GET    | `/stories/top`  | `n` (default 30)                                             | top-N by score       |
| GET    | `/healthz`      | —                                                            | `200 ok`             |
| GET    | `/`             | —                                                            | web frontend         |

`since` / `until` accept RFC3339 or unix seconds. `type` and `q` accept
comma-separated lists. Empty result sets return `[]`, never `null`.

## Configuration (`config.yaml`)

```yaml
brokers: ["localhost:9092"]
topic: "hn-stories"
poll_interval: "60s"
categories: [top, best, show]   # top|new|best|ask|show
filter:                         # consumer-side slice for this instance
  types: [story]
  keywords: [go, rust, ai]
  min_score: 50
```

Env overrides: `KAFKA_BROKERS`, `KAFKA_TOPIC`, `POLL_INTERVAL`. Run multiple
`api` instances with different `filter` blocks to serve different slices of the
same stream.

## Develop

```sh
gofmt -l .
go vet ./...
go build ./...
go test ./...
go test -race ./...
```

## Edge cases handled

Deleted/dead HN items skipped; missing `url`/`score`/`by` defaulted; non-story
types filtered unless requested; duplicate IDs deduped (highest score wins);
per-item fetch failures tolerated (partial result returned); context-based
graceful shutdown; bounded worker pool; retry/backoff on HTTP and Kafka writes;
malformed broker messages skipped; race-detector clean.
