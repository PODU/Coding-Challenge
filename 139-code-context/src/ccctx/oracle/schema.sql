-- ccctx schema for Oracle AI Database 26ai (Free container on port 1521).
-- Mirrors the embedded SQLite schema in store.py table for table.
--
--   docker compose up -d          (see docker-compose.yml in the project root)
--   sqlplus ccctx/ccctx@//localhost:1521/FREEPDB1 @schema.sql
--
-- Run model.sql first: the trigger below calls the ALL_MINILM_L12_V2 model.

CREATE TABLE repos (
    id          NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR2(200)  NOT NULL UNIQUE,
    path        VARCHAR2(4000) NOT NULL,
    commit_sha  VARCHAR2(64),
    embedder    VARCHAR2(100),
    indexed_at  TIMESTAMP,
    created_at  TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL
);

CREATE TABLE files (
    id          NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repo_id     NUMBER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path        VARCHAR2(1000) NOT NULL,
    language    VARCHAR2(40)   NOT NULL,
    content_sha VARCHAR2(40)   NOT NULL,
    bytes       NUMBER         NOT NULL,
    line_count  NUMBER         NOT NULL,
    parser      VARCHAR2(20)   NOT NULL,
    module_doc  CLOB,
    commit_sha  VARCHAR2(64),
    indexed_at  TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL,
    CONSTRAINT files_repo_path UNIQUE (repo_id, path)
);

CREATE TABLE symbols (
    id           NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repo_id      NUMBER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    file_id      NUMBER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    symbol_key   VARCHAR2(1500) NOT NULL,
    name         VARCHAR2(400)  NOT NULL,
    qualname     VARCHAR2(800)  NOT NULL,
    kind         VARCHAR2(20)   NOT NULL,
    language     VARCHAR2(40)   NOT NULL,
    start_line   NUMBER NOT NULL,
    end_line     NUMBER NOT NULL,
    signature    VARCHAR2(1000),
    doc          CLOB,
    source       CLOB NOT NULL,
    search_text  CLOB,                      -- name words + signature + doc + code (what gets embedded)
    content_hash VARCHAR2(40) NOT NULL,
    commit_sha   VARCHAR2(64),
    embedding    VECTOR(384, FLOAT32),
    CONSTRAINT symbols_repo_key UNIQUE (repo_id, symbol_key)
);
CREATE INDEX symbols_name ON symbols (repo_id, name);

-- Embeddings are computed in the database on insert/update with the loaded ONNX model.
CREATE OR REPLACE TRIGGER symbols_embed
BEFORE INSERT OR UPDATE OF search_text ON symbols
FOR EACH ROW
BEGIN
    SELECT VECTOR_EMBEDDING(ALL_MINILM_L12_V2 USING DBMS_LOB.SUBSTR(:NEW.search_text, 4000, 1) AS data)
      INTO :NEW.embedding FROM dual;
END;
/

-- HNSW vector index (needs vector_memory_size > 0, see docker-compose.yml).
CREATE VECTOR INDEX symbols_hnsw ON symbols (embedding)
    ORGANIZATION INMEMORY NEIGHBOR GRAPH
    DISTANCE COSINE
    WITH TARGET ACCURACY 95;

-- Hybrid vector index: Oracle Text + vectors over search_text, for DBMS_HYBRID_VECTOR.SEARCH.
CREATE HYBRID VECTOR INDEX symbols_hybrid ON symbols (search_text)
    PARAMETERS ('MODEL ALL_MINILM_L12_V2 VECTOR_IDXTYPE HNSW');

-- Raw call sites as parsed (callee by name) ...
CREATE TABLE calls (
    src_id   NUMBER NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    dst_name VARCHAR2(400) NOT NULL,
    repo_id  NUMBER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    CONSTRAINT calls_pk PRIMARY KEY (src_id, dst_name)
);
-- ... resolved to symbol -> symbol edges after each ingest (the graph's edge table).
CREATE TABLE call_edges (
    src_id NUMBER NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    dst_id NUMBER NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    CONSTRAINT call_edges_pk PRIMARY KEY (src_id, dst_id)
);

CREATE TABLE imports (
    file_id NUMBER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    repo_id NUMBER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    module  VARCHAR2(1000) NOT NULL,
    CONSTRAINT imports_pk PRIMARY KEY (file_id, module)
);
CREATE TABLE file_imports (
    src_file_id NUMBER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    dst_file_id NUMBER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    CONSTRAINT file_imports_pk PRIMARY KEY (src_file_id, dst_file_id)
);

CREATE TABLE wiki_pages (
    id          NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repo_id     NUMBER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    slug        VARCHAR2(400) NOT NULL,
    module_path VARCHAR2(1000) NOT NULL,
    title       VARCHAR2(400) NOT NULL,
    body        CLOB NOT NULL,
    commit_sha  VARCHAR2(64),
    built_at    TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL,
    embedding   VECTOR(384, FLOAT32),
    CONSTRAINT wiki_repo_slug UNIQUE (repo_id, slug)
);

-- Citations are foreign keys, not prose: staleness is a join.
CREATE TABLE wiki_citations (
    page_id      NUMBER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    symbol_id    NUMBER REFERENCES symbols(id) ON DELETE SET NULL,
    symbol_key   VARCHAR2(1500) NOT NULL,
    role         VARCHAR2(20) NOT NULL,
    content_hash VARCHAR2(40) NOT NULL,
    CONSTRAINT wiki_citations_pk PRIMARY KEY (page_id, symbol_key, role)
);

CREATE OR REPLACE VIEW wiki_staleness AS
SELECT p.id AS page_id, p.slug,
       COUNT(CASE WHEN c.symbol_id IS NULL THEN 1 END)                         AS removed_citations,
       COUNT(CASE WHEN s.content_hash <> c.content_hash THEN 1 END)            AS changed_citations
FROM wiki_pages p
JOIN wiki_citations c ON c.page_id = p.id
LEFT JOIN symbols s   ON s.id = c.symbol_id
GROUP BY p.id, p.slug;

CREATE TABLE runs (
    id           NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ts           TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL,
    tool         VARCHAR2(40) NOT NULL,
    repo         VARCHAR2(200),
    query        VARCHAR2(4000),
    args         JSON,
    results      JSON,
    result_count NUMBER DEFAULT 0 NOT NULL,
    duration_ms  NUMBER
);

CREATE TABLE facts (
    id          NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repo        VARCHAR2(200) NOT NULL,
    subject     VARCHAR2(1000) NOT NULL,
    predicate   VARCHAR2(40) NOT NULL,
    assertion   VARCHAR2(4000) NOT NULL,
    confidence  NUMBER(4,3) NOT NULL,
    source_runs JSON,
    symbol_key  VARCHAR2(1500),
    created_at  TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL,
    updated_at  TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL
);

CREATE TABLE feedback (
    id         NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ts         TIMESTAMP DEFAULT SYSTIMESTAMP NOT NULL,
    repo       VARCHAR2(200) NOT NULL,
    symbol_key VARCHAR2(1500) NOT NULL,
    kind       VARCHAR2(20) NOT NULL,
    run_id     NUMBER REFERENCES runs(id) ON DELETE SET NULL
);
CREATE INDEX feedback_key ON feedback (repo, symbol_key);
