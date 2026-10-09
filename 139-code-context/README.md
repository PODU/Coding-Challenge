# `ccctx` — Code Context MCP server (Coding Challenge #139)

A searchable, graph-aware index of your codebases, served to AI tools over the
Model Context Protocol. It parses code at function/method/class boundaries,
searches it by meaning and by name (hybrid RRF fusion), answers call-graph and
blast-radius questions, writes a cited module wiki that knows when it is stale,
remembers what agents asked and used, and feeds that usage back into ranking.

Python 3.10+, one dependency (`numpy`). The index is an embedded SQLite
database (FTS5 + vectors + recursive-CTE graph), so everything below runs
offline. The Oracle AI Database 26ai version of the same schema and queries
(ONNX model in the database, HNSW, `DBMS_HYBRID_VECTOR.SEARCH`, SQL property
graph) ships as SQL in `src/ccctx/oracle/` — see [Oracle](#oracle) for what
was and wasn't run.

## Install

```sh
pip install -e .            # provides the `ccctx` command
# or, without installing:  PYTHONPATH=src python -m ccctx ...
```

The database lives at `~/.ccctx/ccctx.db` (`$CCCTX_HOME` or `--db` to change it).

## Walkthrough (flask + gin)

```sh
mkdir -p ~/ccctx-repos
git clone https://github.com/pallets/flask.git ~/ccctx-repos/flask
git clone https://github.com/gin-gonic/gin.git ~/ccctx-repos/gin
```

### Step 1 — register

```
$ ccctx repo add ~/ccctx-repos/flask --name flask
$ ccctx repo add ~/ccctx-repos/gin --name gin
$ ccctx repo list
NAME               FILES   SYMBOLS  COMMIT      PATH
flask                  0         0  -           /home/me/ccctx-repos/flask
gin                    0         0  -           /home/me/ccctx-repos/gin
```

### Step 2 — ingest

```
$ ccctx ingest flask
Scanned    236 files
Skipped      5 files (0 ignored, 5 binary, 0 too large)
Indexed  1,382 symbols in 0.7s (1,382 total)
Commit   d73fa1cdcb

$ ccctx symbols --repo flask --language python --limit 3
function  python     github_link                              docs/conf.py:72-97
function  python     setup                                    docs/conf.py:100-101
function  python     create_app                               examples/celery/src/task_app/__init__.py:7-26

$ ccctx langs --repo gin
REPO         LANGUAGE     PARSER       FILES  SYMBOLS
gin          go           structural      97     1515
gin          markdown     fallback         9       86
...
```

`.gitignore` files are honoured at every level (negation, anchoring, `**`,
directory-only rules) along with `.git/info/exclude`. Parsers: **Python**
(stdlib `ast`), **Go, JavaScript, TypeScript, Java, Rust** (structural parser —
strings/comments masked, declarations found per language, bodies brace-matched).
Everything else becomes 60-line windows with a 10-line overlap. Each symbol
stores language, kind, name, qualified name, line range, signature, doc
comment, source, content hash and the commit SHA it was indexed at.

### Step 3 — semantic search

```
$ ccctx search "how are incoming requests routed to handlers" --mode vector
$ ccctx search "kubernetes pod autoscaling policy"
No results.
```

Top 10, cosine similarity ≥ 0.3. Unrelated queries return nothing.

### Step 4 — hybrid search

```
$ ccctx search "url_for" --mode hybrid -k 3
 1. 0.0602  function url_for  [sim 0.37, definition]
      flask/src/flask/helpers.py:200-251
 2. 0.0472  method   Flask.url_for  [definition]
      flask/src/flask/app.py:1105-1225
 3. 0.0258  method   App.handle_url_build_error  [sim 0.38]
      flask/src/flask/sansio/app.py:981-1013

$ ccctx search "middleware chain" --repo gin -k 3
 1. 0.0318  function Use  [sim 0.48]
      gin/ginS/gins.go:129-131
 2. 0.0313  method   Engine.Use  [sim 0.38]
      gin/gin.go:363-368
 3. 0.0290  method   RouterGroup.Use  [sim 0.31]
      gin/routergroup.go:89-92

$ ccctx search "url_for" --repo flask --compare -k 2
== vector
 1. method   Scaffold.static_url_path                   flask/src/flask/sansio/scaffold.py:264-269
 2. method   Scaffold.static_url_path                   flask/src/flask/sansio/scaffold.py:248-262
== text
 1. function url_for                                    flask/src/flask/helpers.py:200-251
 2. method   Flask.url_for                              flask/src/flask/app.py:1105-1225
== hybrid
 1. function url_for                                    flask/src/flask/helpers.py:200-251
 2. method   Flask.url_for                              flask/src/flask/app.py:1105-1225
```

Fusion is Reciprocal Rank Fusion (k = 60) in UNION mode, plus an
exact-definition list so definitions outrank call sites. The gold set
(`gold/flask.json`, 18 questions + 4 negatives) shows hybrid beating both
single modes:

| mode | hit@10 | recall@10 | MRR | results on negatives |
|------|-------:|----------:|----:|---------------------:|
| vector | 0.833 | 0.806 | 0.407 | 1 |
| text | 0.889 | 0.861 | 0.381 | 22 |
| **hybrid** | **0.944** | **0.917** | **0.641** | 1 |

### Step 5 — incremental re-indexing

```
$ ccctx ingest flask
Up to date at d73fa1cdcb (1,382 symbols) - nothing to do

$ git -C ~/ccctx-repos/flask checkout HEAD~20 && ccctx ingest flask
Scanned    235 files
Skipped      5 files (0 ignored, 5 binary, 0 too large)
Changed     35 files (+0 added, ~34 modified, -1 deleted, >0 renamed; 196 unchanged)
Indexed    123 symbols in 1.0s (1,378 total)
Commit   3a9d54f3da (was d73fa1cdcb)

$ ccctx ingest flask --full        # drop and rebuild
```

Only added, modified and renamed files are parsed, and inside a modified file
only symbols whose code changed are re-embedded (123 of 1,378 above). Deleted
files take their symbols, embeddings, FTS rows and call edges with them.
`-v` lists the changed files.

### Step 6 — call graph

```
$ ccctx callers url_for --repo flask
36 caller(s) of url_for:
  login_required                               function flask/examples/tutorial/flaskr/auth.py:19
  ...
$ ccctx calls Flask.url_for --repo flask
$ ccctx impact url_for --repo flask --depth 3
Changing url_for may affect 154 symbol(s) in 37 file(s) (36 at hop 1, 40 at hop 2, 78 at hop 3):
  [1] login_required                               function flask/examples/tutorial/flaskr/auth.py:19
  ...
```

Calls are resolved by name (no type checker), so same-named methods are grouped.

### Step 7 — MCP server

```sh
ccctx mcp                                              # JSON-RPC 2.0 over stdio
npx @modelcontextprotocol/inspector ccctx mcp          # interactive
npx @modelcontextprotocol/inspector --cli ccctx mcp --method tools/list
```

Tools: `search_code` (query, repo, language, k, mode), `get_symbol`,
`find_callers`, `impact_of`, `read_wiki`, `recall`. Descriptions say *when*
to use each one ("use this FIRST…", "call get_symbol to read a result…",
"prefer find_callers when you only need direct call sites"). To use it from
Claude Code:

```sh
claude mcp add ccctx -- ccctx mcp
```

### Step 8 — wiki

```
$ ccctx wiki build --repo flask
flask: built 7, fresh 0, removed 0
$ ccctx wiki show flask/json
$ ccctx wiki citations flask/json
18 citation(s) for flask/json (built at commit d73fa1cdcb):
  fresh    key    #6631   src/flask/json/__init__.py::dumps
  ...
$ ccctx search "what does the json module do" --repo flask -k 1
 1. 0.0588  wiki     flask/json  [sim 0.32]
```

One page per directory with ≥ 10 symbols: purpose (module docstring),
key symbols, entry points used by other modules, imports / imported-by.
Citations are rows with a foreign key to `symbols` and the content hash at
build time, so `citations` reports `fresh` / `changed` / `missing`, and
`wiki build` (also run automatically after `ingest`) recompiles only stale pages.

### Step 9 — memory

```
$ ccctx memory list --repo flask --limit 10
#24    2026-10-04 17:24:15  get_symbol    flask    'url_for'      1 -> src/flask/app.py::Flask.url_for
...
$ ccctx memory consolidate --repo flask
$ ccctx memory facts --repo flask
0.75  frequently_used Flask.url_for (src/flask/app.py) is consulted often (7 uses)  (runs [7, 9, 16, 18, 22])
0.65  answers         "url_for" is answered by Flask.url_for (src/flask/app.py)  (runs [16])
0.55  gap             Nothing indexed matches "nonexistent quantum teleportation"  (runs [25])
```

Every tool call (CLI or MCP) is a run. Consolidation (automatic after each
re-index) turns runs + feedback into facts with confidence and source run
ids, retires facts about deleted symbols, and `answers` facts give a small
boost to the same symbol on later, similar searches. `recall` (MCP tool and
`ccctx memory recall`) returns relevant facts plus recent runs.

### Step 10 — feedback-weighted ranking

`get_symbol` records an "open" for what it returns, linked to the search
that surfaced it; `ccctx feedback <symbol>` records a citation. The boost is
`0.20 * (1 - exp(-w/3))` where `w` sums `0.5 ** (age_days / 30)` over events
— capped below +20%, with a 30-day half-life — and is applied only to
documents the retrievers already returned.

```
$ ccctx eval --gold gold/flask.json --with-feedback
mode               hit@k  recall@k    MRR  neg results  neg boosted
vector             0.833     0.806  0.407            1            0
text               0.889     0.861  0.381           22            0
hybrid             0.944     0.917  0.641            1            0
hybrid+feedback    1.000     0.972  0.819            1            0
(simulated 56 feedback events on an in-memory copy; k=10)
```

`--with-feedback` simulates an agent on an in-memory copy of the database
(your real feedback is untouched): it pages through 3×k results, opens the
gold answers, then re-scores. Note this evaluates on the same questions it
trained on, so it shows the mechanism works, not how well it generalises.
"neg boosted" counts boosted results on the negative questions — 0 means
popular symbols didn't leak into unrelated queries.

## Command reference

| Command | |
|---|---|
| `repo add <path> [--name N]` / `repo list` / `repo remove N` | register repositories |
| `ingest <name> [--full] [-v]` / `ingest --all` | index / re-index |
| `symbols [--repo] [--kind] [--language] [--file] [--name] [--limit]` | list symbols |
| `langs [--repo]` | language & parser breakdown |
| `search <q> [--mode hybrid\|vector\|text] [--repo] [--language] [-k] [--compare] [--json]` | search |
| `get <name>` | full source of a symbol |
| `callers <sym>` / `calls <sym>` / `impact <sym> [--depth 3]` | call graph |
| `mcp` | MCP server on stdio |
| `wiki build [--repo] [--force]` / `wiki list` / `wiki show <slug>` / `wiki citations <slug>` | wiki |
| `memory list` / `memory consolidate` / `memory facts` / `memory recall <q>` | memory |
| `feedback <sym> [--kind cite]` | record usage |
| `eval --gold F [--with-feedback] [-k] [-v] [--json]` | retrieval evaluation |
| `oracle [all\|schema\|model\|search\|graph]` | print the Oracle 26ai SQL |
| `stats` | table counts |

## Embeddings

The default embedder (`hash-v4`) is deterministic and offline: a 384-d
feature-hashed bag of code-aware terms (identifier splitting, light stemming,
code/English stopwords, a small table of programming synonyms). It is a
lexical model, not a neural one — it can't know that "generate" means
"build" unless the synonym table says so — which is why the vector column
alone scores lowest above. `OnnxEmbedder` runs the real `all-MiniLM-L12-v2`
locally if `onnxruntime` and `tokenizers` are installed
(`CCCTX_EMBEDDER=onnx CCCTX_ONNX_MODEL=<dir with model.onnx + tokenizer.json>`);
it was not exercised in this build. Changing embedder forces a full re-index.

## Oracle

`ccctx oracle` prints the 26ai version: `DBMS_VECTOR.LOAD_ONNX_MODEL` for
`ALL_MINILM_L12_V2`, `VECTOR(384, FLOAT32)` columns filled by a
`VECTOR_EMBEDDING` trigger, an HNSW index
(`ORGANIZATION INMEMORY NEIGHBOR GRAPH DISTANCE COSINE WITH TARGET ACCURACY 95`),
a hybrid vector index queried with `DBMS_HYBRID_VECTOR.SEARCH` (`rrf`, `UNION`),
and a `CREATE PROPERTY GRAPH` with `GRAPH_TABLE` queries for callers, calls and
the 1..3-hop blast radius. `docker-compose.yml` + `oracle-init/` bring up
Oracle AI Database Free on port 1521 with persistent storage.

**Not verified:** Docker wasn't running on the build machine, so these
scripts have not been run against a live database, and `ccctx` does not have
an Oracle connection backend — the SQLite backend implements the same schema
and query semantics. Expect small syntax fixes when you first run them.

## Tests

```sh
python -m pytest -q        # 80 tests, offline, ~20s
```

Fixture repo with Python, Go, TypeScript, Markdown, a `.gitignore`, a
binary, and a git history. Covers gitignore semantics, every parser,
incremental ingest (modify / delete / rename / no-op / full / embedder change),
search thresholds and fusion, definition ranking, bounded + decaying feedback,
graph hops and cycles, wiki citations and staleness, memory consolidation,
MCP protocol (lifecycle, tools, errors, batch, stdio), the CLI, and eval.
