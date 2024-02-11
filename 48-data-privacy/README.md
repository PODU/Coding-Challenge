# Data Privacy Vault

A tokenization HTTP API: callers send sensitive fields (PII) to `/tokenize` and
get back opaque **tokens** they can safely store, log, and share. To recover the
originals they call `/detokenize`. The vault is the only component that ever
holds plaintext, and it holds it **encrypted at rest** (AES-256-GCM) behind
**bearer authentication with per-endpoint scopes**.

This implements Coding Challenge #48.

## Features

- `POST /tokenize` — `id` + `data` fields → tokens (HTTP **201**).
- `POST /detokenize` — tokens → original values, with a per-field
  **found / not-found** status (HTTP **200**, never fails the batch on a miss).
- **Encryption at rest** — AES-256-GCM AEAD, fresh 96-bit nonce per record, and
  the token bound in as additional authenticated data (AAD) so a ciphertext
  cannot be transplanted under another token. A versioned keyring supports key
  rotation (old keys retained decrypt-only).
- **Pluggable persistent storage** behind a `Store` interface:
  `bolt` (default, embedded single-file [bbolt]), `redis` (optional adapter),
  or `memory`. Only ciphertext is ever written to a backend.
- **Auth & authz** — bearer API keys mapped to scopes (`tokenize`,
  `detokenize`). Keys are stored as SHA-256 hashes and compared in constant time.
  Missing/invalid key → `401`; valid key without the required scope → `403`.
- **HTTPS** — native TLS when `TLS_CERT`/`TLS_KEY` are set (HSTS sent over TLS);
  otherwise terminate TLS at a reverse proxy.
- Format-preserving tokens (optional), deterministic tokenization (optional),
  request IDs, structured access logs (never logging plaintext), panic recovery,
  body-size cap, strict content-type, and graceful shutdown.

## Requirements

- Go 1.25+ (bbolt v1.5 requires Go ≥ 1.25).

## Build

```sh
go build -o vault ./cmd/vault    # or: make build
```

## Quick start

```sh
# 1. Generate a master key (base64 of 32 bytes) — never hardcode this.
export VAULT_MASTER_KEY="$(go run ./cmd/vault genkey)"

# 2. Provide at least one API key with scopes (inline JSON for dev).
export API_KEYS='[{"name":"svc","key":"dev-secret","scopes":["tokenize","detokenize"]}]'

# 3. Run (memory backend, plain HTTP, for local testing).
export STORE_BACKEND=memory
export ADDR=127.0.0.1:8443
go run ./cmd/vault serve
```

Tokenize:

```sh
curl -s -X POST http://127.0.0.1:8443/tokenize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-secret" \
  -d '{"id":"customer-42","data":{"ssn":"123-45-6789","email":"alice@example.com"}}'
# 201 Created
# {"id":"customer-42","data":{"ssn":"tok_...","email":"tok_..."}}
```

Detokenize (note the per-field found/not-found):

```sh
curl -s -X POST http://127.0.0.1:8443/detokenize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-secret" \
  -d '{"tokens":["tok_...","tok_unknown"]}'
# 200 OK
# {"results":{"tok_...":{"found":true,"value":"123-45-6789"},"tok_unknown":{"found":false}}}
```

## CLI

```
vault serve                          start the HTTP(S) API (reads env)
vault genkey                         print a fresh base64 32-byte master key
vault apikey NAME --scopes a,b       mint an API key; prints the secret once and
                                     the JSON entry (only the hash) to configure
```

`vault apikey` prints the bearer secret **once** and a JSON entry containing only
the SHA-256 hash — store the entry in `API_KEYS`/`API_KEYS_FILE`, never the
plaintext key.

## HTTP API

| Method | Path          | Auth scope    | Success |
|--------|---------------|---------------|---------|
| POST   | `/tokenize`   | `tokenize`    | 201     |
| POST   | `/detokenize` | `detokenize`  | 200     |
| GET    | `/healthz`    | none          | 200     |
| GET    | `/readyz`     | none (pings store) | 200 |

`POST /tokenize` request:

```json
{ "id": "customer-42",
  "data": { "ssn": "123-45-6789", "email": "alice@example.com" },
  "options": { "format_preserving": false, "deterministic": false } }
```

- `format_preserving` — emit tokens matching the detected input shape
  (phone/email/SSN/credit-card-with-valid-Luhn). The shaped token is a random
  value cast into the shape, never a reversible transform of the input.
- `deterministic` — identical plaintext maps to the same token across calls (via
  an HMAC dedup index that stores **no** plaintext). Off by default; random
  tokens are the privacy-safest.

Errors use a uniform envelope:

```json
{ "error": { "code": "unauthorized", "message": "...", "request_id": "..." } }
```

| Status | When |
|--------|------|
| 400 | malformed JSON, missing `data`/`tokens`, oversized body, value too large |
| 401 | missing/invalid bearer token |
| 403 | valid key lacks the required scope |
| 415 | `Content-Type` is not `application/json` |
| 500 | internal error (never leaks plaintext or crypto detail) |

## Environment variables

| Variable | Default | Notes |
|----------|---------|-------|
| `ADDR` | `:8443` | listen address |
| `STORE_BACKEND` | `bolt` | `memory` \| `bolt` \| `redis` |
| `BOLT_PATH` | `vault.db` | bbolt file path (bolt backend) |
| `REDIS_ADDR` | `localhost:6379` | redis backend |
| `REDIS_PASSWORD` | – | redis auth (optional) |
| `REDIS_DB` | `0` | redis db number |
| `VAULT_MASTER_KEY` | **required** | base64 of 32 bytes (AES-256). Generate with `vault genkey`. |
| `VAULT_DEDUP_KEY` | derived | base64 of 32 bytes; HMAC key for deterministic mode. Derived from the master key if unset. |
| `API_KEYS` | – | inline JSON array of key entries (takes precedence over the file) |
| `API_KEYS_FILE` | – | path to a JSON array of key entries |
| `TLS_CERT` / `TLS_KEY` | – | enable HTTPS when both are set |

API key entry format (use `key_sha256` in production; `key` plaintext is a dev
convenience that is hashed at load and never retained):

```json
[ { "name": "logger",  "key_sha256": "…64 hex…", "scopes": ["tokenize"] },
  { "name": "support", "key_sha256": "…64 hex…", "scopes": ["tokenize","detokenize"] } ]
```

## Production / HTTPS

Run with TLS directly:

```sh
export VAULT_MASTER_KEY=...           # from a secrets manager / KMS, not the shell history
export STORE_BACKEND=bolt BOLT_PATH=/var/lib/vault/vault.db
export API_KEYS_FILE=/etc/vault/keys.json
export TLS_CERT=/etc/vault/tls.crt TLS_KEY=/etc/vault/tls.key
export ADDR=:8443
vault serve
```

Or terminate TLS at a reverse proxy (nginx/Envoy/ALB) and run the vault on a
private network. A `Dockerfile` is provided for containerized deploys.

## Security notes

- **Key management** — the master key comes only from the environment (or a KMS
  hook); it is never hardcoded or defaulted. Rotate by adding a new key version
  to the keyring; existing records carry their `KeyVersion` and remain readable.
- **Encrypted at rest** — backends only ever see `{nonce, ciphertext+tag}`. A
  test (`internal/vault`) scans the raw bbolt file and fails if any plaintext PII
  appears.
- **Least privilege** — give a logging/ingest service only the `tokenize` scope
  so it can never read plaintext back; restrict `detokenize` to trusted callers.
- **No plaintext in logs** — access logs record method/path/status/timing only.
- **Constant-time auth** — API keys are matched with `crypto/subtle` to avoid a
  timing oracle.
- **Reversible by design** — tokenization here is reversible (that is the point).
  For irreversible needs, a one-way HMAC redaction is the right tool.

## Testing

```sh
go test ./...                 # all unit + HTTP integration tests
go test -race ./...           # add the race detector (needs a 64-bit C toolchain)
go test -run=NONE -fuzz=FuzzTokenizeRoundTrip -fuzztime=30s ./internal/vault
```

Covered: crypto round-trip / tamper / wrong-key / rotation; token shapes + Luhn;
store interface against memory **and** bolt incl. persistence across reopen;
tokenize↔detokenize round-trip; not-found + duplicate handling; deterministic
mode; encryption-at-rest on-disk scan; HTTP 201/200, auth matrix
(401/403/2xx), malformed JSON (400), wrong content-type (415), oversized body
(400); TLS smoke test; and a round-trip fuzz target.

[bbolt]: https://github.com/etcd-io/bbolt
