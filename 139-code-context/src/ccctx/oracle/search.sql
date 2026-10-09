-- Search queries (bind variables: :query, :repo, :k).

-- 1. Semantic search: top 10 by cosine similarity, similarity >= 0.3.
SELECT id, qualname, kind, file_id, start_line, end_line, similarity
FROM (
    SELECT s.id, s.qualname, s.kind, s.file_id, s.start_line, s.end_line,
           1 - VECTOR_DISTANCE(s.embedding,
                               VECTOR_EMBEDDING(ALL_MINILM_L12_V2 USING :query AS data),
                               COSINE) AS similarity
    FROM symbols s JOIN repos r ON r.id = s.repo_id
    WHERE (:repo IS NULL OR r.name = :repo)
    ORDER BY VECTOR_DISTANCE(s.embedding,
                             VECTOR_EMBEDDING(ALL_MINILM_L12_V2 USING :query AS data), COSINE)
    FETCH APPROX FIRST 50 ROWS ONLY WITH TARGET ACCURACY 95
)
WHERE similarity >= 0.3
ORDER BY similarity DESC
FETCH FIRST 10 ROWS ONLY;

-- 2. Hybrid search: vector + Oracle Text fused with Reciprocal Rank Fusion, UNION mode.
SELECT JSON_SERIALIZE(
    DBMS_HYBRID_VECTOR.SEARCH(
        JSON('{
            "hybrid_index_name": "SYMBOLS_HYBRID",
            "search_scorer"    : "rrf",
            "search_fusion"    : "UNION",
            "vector"           : { "search_text": "' || :query || '", "search_mode": "DOCUMENT" },
            "text"             : { "contains": "' || :query || '" },
            "return"           : { "topN": 50, "values": ["rowid", "score", "vector_score", "text_score"] }
        }'))
    PRETTY) AS results
FROM dual;

-- 3. Exact-definition boost (the third RRF list): definitions whose name is in the query rank
--    above call sites. Fuse with the hybrid results by rowid in the application layer.
SELECT s.id, s.qualname,
       2 / (60 + ROW_NUMBER() OVER (ORDER BY CASE s.kind WHEN 'function' THEN 0 WHEN 'class' THEN 0
                                                         WHEN 'method' THEN 1 ELSE 2 END,
                                             LENGTH(s.qualname))) AS rrf_definition
FROM symbols s JOIN repos r ON r.id = s.repo_id
WHERE (s.name = :identifier OR s.qualname = :identifier)
  AND s.kind <> 'chunk'
  AND (:repo IS NULL OR r.name = :repo);

-- 4. Bounded feedback boost with a 30-day half-life (multiply the fused score by 1 + boost).
SELECT symbol_key,
       0.20 * (1 - EXP(-SUM(POWER(0.5, (CAST(SYSTIMESTAMP AS DATE) - CAST(ts AS DATE)) / 30)) / 3)) AS boost
FROM feedback
WHERE repo = :repo
GROUP BY symbol_key;
