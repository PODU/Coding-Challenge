-- SQL property graph over the code index, and the traversals behind
-- `ccctx callers`, `ccctx calls` and `ccctx impact`.

-- Resolve parsed call sites (callee by name) into symbol -> symbol edges.
DELETE FROM call_edges WHERE src_id IN (SELECT id FROM symbols WHERE repo_id = :repo_id);
INSERT INTO call_edges (src_id, dst_id)
SELECT DISTINCT c.src_id, d.id
FROM calls c JOIN symbols d ON d.repo_id = c.repo_id AND d.name = c.dst_name AND d.kind <> 'chunk'
WHERE c.repo_id = :repo_id;

CREATE OR REPLACE PROPERTY GRAPH code_graph
    VERTEX TABLES (
        files   KEY (id) LABEL file   PROPERTIES (id, repo_id, path, language),
        symbols KEY (id) LABEL symbol PROPERTIES (id, repo_id, name, qualname, kind, file_id,
                                                  start_line, end_line)
    )
    EDGE TABLES (
        call_edges
            KEY (src_id, dst_id)
            SOURCE      KEY (src_id) REFERENCES symbols (id)
            DESTINATION KEY (dst_id) REFERENCES symbols (id)
            LABEL calls NO PROPERTIES,
        symbols AS defines
            KEY (id)
            SOURCE      KEY (file_id) REFERENCES files (id)
            DESTINATION KEY (id)      REFERENCES symbols (id)
            LABEL defines NO PROPERTIES,
        file_imports
            KEY (src_file_id, dst_file_id)
            SOURCE      KEY (src_file_id) REFERENCES files (id)
            DESTINATION KEY (dst_file_id) REFERENCES files (id)
            LABEL imports NO PROPERTIES
    );

-- callers url_for
SELECT caller, caller_kind, path, line
FROM GRAPH_TABLE (code_graph
    MATCH (f IS file) -[IS defines]-> (a IS symbol) -[IS calls]-> (b IS symbol WHERE b.name = :name)
    COLUMNS (a.qualname AS caller, a.kind AS caller_kind, f.path AS path, a.start_line AS line))
ORDER BY path, line;

-- calls url_for
SELECT DISTINCT callee, callee_kind
FROM GRAPH_TABLE (code_graph
    MATCH (a IS symbol WHERE a.name = :name OR a.qualname = :name) -[IS calls]-> (b IS symbol)
    COLUMNS (b.qualname AS callee, b.kind AS callee_kind))
ORDER BY callee;

-- impact url_for --depth 3: everything that reaches the target within 1..3 hops,
-- with the shortest hop count for each affected symbol.
SELECT affected, MIN(hops) AS hops
FROM GRAPH_TABLE (code_graph
    MATCH (a IS symbol) -[e IS calls]->{1,3} (b IS symbol WHERE b.name = :name)
    COLUMNS (a.qualname AS affected, COUNT(e.src_id) AS hops))
GROUP BY affected
ORDER BY hops, affected;

-- Module dependencies for the wiki: which files does a module import?
SELECT DISTINCT dst
FROM GRAPH_TABLE (code_graph
    MATCH (x IS file WHERE x.path LIKE :module || '/%') -[IS imports]-> (y IS file)
    COLUMNS (y.path AS dst));
