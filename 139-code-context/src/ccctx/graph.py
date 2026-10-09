"""Call graph and dependency queries.

Vertices are symbols and files; edges are `calls` (symbol -> callee name,
resolved to symbols of that name in the same repository), `defines`
(file -> symbol, implicit in symbols.file_id) and `imports` (file -> module).
Calls are resolved by name, as most tools without a type checker do, so
`callers render` covers every function
named render in the repo. Qualified names (`Flask.url_for`) narrow the
target to one definition, but callers are still matched by the short name.

Oracle runs the same traversals as GRAPH_TABLE queries over a SQL property
graph (see oracle/graph.sql); here they are recursive CTEs over the same
tables.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass
from pathlib import PurePosixPath

from .store import Store


@dataclass
class GraphHit:
    key: str
    repo: str
    path: str
    name: str
    qualname: str
    kind: str
    start_line: int
    end_line: int
    hops: int = 1
    via: str = ""  # the name that links this symbol to the previous hop

    def to_dict(self):
        return asdict(self)

    @property
    def location(self):
        return f"{self.repo}/{self.path}:{self.start_line}"


_HIT_SQL = ("SELECT s.key, rp.name AS repo, f.path, s.name, s.qualname, s.kind, s.start_line, s.end_line "
            "FROM symbols s JOIN files f ON f.id = s.file_id JOIN repos rp ON rp.id = s.repo_id ")


def _repo_ids(store: Store, repo: str | None) -> list[int]:
    if repo:
        r = store.repo(repo)
        if r is None:
            raise KeyError(f"unknown repository '{repo}'")
        return [r["id"]]
    return [r[0] for r in store.q("SELECT id FROM repos")]


def resolve(store: Store, symbol: str, repo: str | None = None) -> list[GraphHit]:
    """Definitions matching a name, qualified name or symbol key."""
    ids = _repo_ids(store, repo)
    marks = ",".join("?" * len(ids)) or "NULL"
    rows = store.q(_HIT_SQL + f"WHERE s.repo_id IN ({marks}) AND (s.name = ? OR s.qualname = ? OR s.key = ?) "
                   "AND s.kind != 'chunk' ORDER BY rp.name, instr(s.qualname, '.') > 0, "
                   "f.path LIKE '%test%', f.path, s.start_line", [*ids, symbol, symbol, symbol])
    return [GraphHit(*r, hops=0) for r in rows]


def _short(symbol: str) -> str:
    return symbol.split("::")[-1].split(".")[-1].split("#")[0]


def callers(store: Store, symbol: str, repo: str | None = None) -> list[GraphHit]:
    ids = _repo_ids(store, repo)
    marks = ",".join("?" * len(ids)) or "NULL"
    rows = store.q(_HIT_SQL + f"JOIN calls c ON c.src_id = s.id WHERE c.repo_id IN ({marks}) AND c.dst_name = ? "
                   "ORDER BY rp.name, f.path, s.start_line", [*ids, _short(symbol)])
    return [GraphHit(*r, hops=1, via=_short(symbol)) for r in rows]


def calls(store: Store, symbol: str, repo: str | None = None) -> tuple[list[GraphHit], list[str]]:
    """What `symbol` calls: (resolved in-repo definitions, unresolved external names)."""
    targets = resolve(store, symbol, repo)
    if not targets:
        return [], []
    keys = [t.key for t in targets]
    kmarks = ",".join("?" * len(keys))
    names = [r[0] for r in store.q(
        f"SELECT DISTINCT c.dst_name FROM calls c JOIN symbols s ON s.id = c.src_id "
        f"WHERE s.key IN ({kmarks}) ORDER BY c.dst_name", keys)]
    repo_ids = sorted({store.repo(t.repo)["id"] for t in targets})
    rmarks = ",".join("?" * len(repo_ids))
    resolved: list[GraphHit] = []
    found: set[str] = set()
    for i in range(0, len(names), 500):
        chunk = names[i:i + 500]
        nmarks = ",".join("?" * len(chunk))
        for r in store.q(_HIT_SQL + f"WHERE s.repo_id IN ({rmarks}) AND s.name IN ({nmarks}) "
                         "AND s.kind != 'chunk' ORDER BY s.name, f.path", [*repo_ids, *chunk]):
            hit = GraphHit(*r, hops=1, via=r[3])
            resolved.append(hit)
            found.add(hit.name)
    external = [n for n in names if n not in found]
    return resolved, external


def impact(store: Store, symbol: str, repo: str | None = None, depth: int = 3) -> list[GraphHit]:
    """Blast radius: every symbol that reaches `symbol` through <= depth call hops."""
    ids = _repo_ids(store, repo)
    marks = ",".join("?" * len(ids)) or "NULL"
    rows = store.q(f"""
        WITH RECURSIVE reach(sid, name, hops, via) AS (
            SELECT s.id, s.name, 1, c.dst_name
            FROM calls c JOIN symbols s ON s.id = c.src_id
            WHERE c.dst_name = ? AND c.repo_id IN ({marks})
            UNION
            SELECT s.id, s.name, r.hops + 1, c.dst_name
            FROM reach r JOIN calls c ON c.dst_name = r.name AND c.repo_id IN ({marks})
            JOIN symbols s ON s.id = c.src_id
            WHERE r.hops < ?
        )
        SELECT s.key, rp.name, f.path, s.name, s.qualname, s.kind, s.start_line, s.end_line,
               MIN(r.hops) AS hops, r.via
        FROM reach r JOIN symbols s ON s.id = r.sid JOIN files f ON f.id = s.file_id
        JOIN repos rp ON rp.id = s.repo_id
        GROUP BY s.id ORDER BY hops, rp.name, f.path, s.start_line""",
                   [_short(symbol), *ids, *ids, depth])
    return [GraphHit(r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], hops=r[8], via=r[9]) for r in rows]



def module_of(path: str) -> str:
    """The module a file belongs to: its directory (the package)."""
    parent = PurePosixPath(path).parent.as_posix()
    return "" if parent == "." else parent


def resolve_import(module: str, from_path: str, language: str, file_paths: set[str],
                   go_module: str | None = None) -> str | None:
    """Map an import string to a repo file path, if it refers to one."""
    here = PurePosixPath(from_path).parent
    if language == "python":
        level = len(module) - len(module.lstrip("."))
        parts = [p for p in module.lstrip(".").split(".") if p]
        if level:
            base = here
            for _ in range(level - 1):
                base = base.parent
            stem = (base.joinpath(*parts) if parts else base).as_posix()
            for cand in (f"{stem}.py", f"{stem}/__init__.py"):
                if cand in file_paths:
                    return cand
            return None
        suffixes = ["/".join(parts) + ".py", "/".join(parts) + "/__init__.py"]
        for p in file_paths:
            for suf in suffixes:
                if p == suf or p.endswith("/" + suf):
                    return p
        return None
    if language == "go":
        if go_module and (module == go_module or module.startswith(go_module + "/")):
            rel = module[len(go_module):].lstrip("/")
            for p in file_paths:
                if p.endswith(".go") and module_of(p) == rel:
                    return p
        return None
    if language in ("javascript", "typescript") and module.startswith("."):
        base = (here / module).as_posix()
        norm = PurePosixPath(base)
        stack: list[str] = []
        for part in norm.parts:
            if part == "..":
                if stack:
                    stack.pop()
            elif part != ".":
                stack.append(part)
        stem = "/".join(stack)
        for ext in ("", ".ts", ".tsx", ".js", ".jsx", ".mjs", "/index.ts", "/index.js"):
            if stem + ext in file_paths:
                return stem + ext
    return None


def file_dependencies(store: Store, repo: str) -> dict[str, set[str]]:
    """{file path: set of in-repo file paths it imports}."""
    r = store.repo(repo)
    files = {row["path"]: row for row in store.q("SELECT id, path, language FROM files WHERE repo_id = ?",
                                                 (r["id"],))}
    paths = set(files)
    go_module = None
    gomod = next((p for p in paths if p == "go.mod"), None)
    if gomod:
        try:
            from pathlib import Path
            text = (Path(r["path"]) / gomod).read_text(encoding="utf-8", errors="replace")
            for line in text.splitlines():
                if line.startswith("module "):
                    go_module = line.split()[1]
        except OSError:
            pass
    out: dict[str, set[str]] = {}
    by_id = {row["id"]: row for row in files.values()}
    for fid, module in store.q("SELECT file_id, module FROM imports WHERE repo_id = ?", (r["id"],)):
        row = by_id.get(fid)
        if not row:
            continue
        target = resolve_import(module, row["path"], row["language"], paths, go_module)
        if target and target != row["path"]:
            out.setdefault(row["path"], set()).add(target)
    return out
