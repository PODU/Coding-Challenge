"""Ingestion: walk a repository, parse files into symbols, embed and store them.

Re-ingestion is incremental. A repository whose HEAD has not moved and whose
working tree is clean is skipped outright. Otherwise every file's content
hash is compared with the stored one, so only added, modified and renamed
files are touched, and rows for deleted files are removed (their symbols,
embeddings, FTS entries and call edges go with them). Inside a changed file
symbols are matched by a stable key (path::qualname), so an unchanged
function keeps its row id. That is what lets wiki citations (foreign keys
to symbols) survive edits elsewhere in the file and go stale only when the
cited code itself changes.
"""

from __future__ import annotations

import hashlib
import time
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path

from . import gitutil
from .embed import get_embedder, split_identifier, symbol_text, to_blob
from .gitignore import walk_repo
from .langs import detect
from .parsers import parse
from .store import Store


@dataclass
class IngestReport:
    repo: str
    scanned: int = 0
    ignored: int = 0
    binary: int = 0
    too_large: int = 0
    added: list[str] = field(default_factory=list)
    modified: list[str] = field(default_factory=list)
    deleted: list[str] = field(default_factory=list)
    renamed: list[tuple[str, str]] = field(default_factory=list)
    unchanged: int = 0
    symbols_indexed: int = 0
    symbols_removed: int = 0
    symbols_total: int = 0
    seconds: float = 0.0
    commit_sha: str | None = None
    previous_sha: str | None = None
    up_to_date: bool = False
    full: bool = False
    incremental: bool = False
    languages: Counter = field(default_factory=Counter)
    notes: list[str] = field(default_factory=list)

    @property
    def skipped(self) -> int:
        return self.ignored + self.binary + self.too_large

    @property
    def changed(self) -> int:
        return len(self.added) + len(self.modified) + len(self.deleted) + len(self.renamed)

    def render(self) -> str:
        if self.up_to_date:
            return (f"Up to date at {gitutil.short(self.commit_sha)} "
                    f"({self.symbols_total:,} symbols) - nothing to do")
        lines = [
            f"Scanned  {self.scanned:>5,} files",
            f"Skipped  {self.skipped:>5,} files ({self.ignored} ignored, {self.binary} binary, "
            f"{self.too_large} too large)",
        ]
        if self.incremental:
            lines.append(f"Changed  {self.changed:>5,} files (+{len(self.added)} added, "
                         f"~{len(self.modified)} modified, -{len(self.deleted)} deleted, "
                         f">{len(self.renamed)} renamed; {self.unchanged} unchanged)")
        lines.append(f"Indexed {self.symbols_indexed:>6,} symbols in {self.seconds:.1f}s "
                     f"({self.symbols_total:,} total)")
        if self.commit_sha:
            moved = (f" (was {gitutil.short(self.previous_sha)})"
                     if self.previous_sha and self.previous_sha != self.commit_sha else "")
            lines.append(f"Commit   {gitutil.short(self.commit_sha)}{moved}")
        lines.extend(f"Note     {n}" for n in self.notes)
        return "\n".join(lines)


def content_sha(data: bytes) -> str:
    return hashlib.sha1(data).hexdigest()


def _symbol_hash(sym) -> str:
    return hashlib.sha1(f"{sym.kind}\0{sym.qualname}\0{sym.source}".encode()).hexdigest()


def ingest(store: Store, name: str, full: bool = False, embedder=None,
           max_bytes: int = 1_000_000, consolidate: bool = True) -> IngestReport:
    repo = store.repo(name)
    if repo is None:
        raise KeyError(f"unknown repository '{name}' (add it with: ccctx repo add <path> --name {name})")
    root = Path(repo["path"])
    if not root.is_dir():
        raise FileNotFoundError(f"repository path {root} does not exist")

    t0 = time.perf_counter()
    embedder = embedder or get_embedder()
    report = IngestReport(name, full=full, previous_sha=repo["commit_sha"])
    head = gitutil.head(root)
    report.commit_sha = head
    repo_id = repo["id"]

    if repo["embedder"] and repo["embedder"] != embedder.name and not full:
        report.notes.append(f"embedder changed ({repo['embedder']} -> {embedder.name}); doing a full rebuild")
        full = report.full = True

    if (not full and head and repo["commit_sha"] == head and repo["indexed_at"]
            and not gitutil.is_dirty(root)):
        report.up_to_date = True
        report.symbols_total = store.q1("SELECT COUNT(*) FROM symbols WHERE repo_id = ?", (repo_id,))[0]
        report.seconds = time.perf_counter() - t0
        return report

    if full:
        ids = [r[0] for r in store.q("SELECT id FROM symbols WHERE repo_id = ?", (repo_id,))]
        report.symbols_removed = len(ids)
        store._fts_delete(ids)
        store.db.execute("DELETE FROM files WHERE repo_id = ?", (repo_id,))

    stats, contents = walk_repo(root, max_bytes=max_bytes)
    report.scanned, report.ignored = stats.scanned, stats.ignored
    report.binary, report.too_large = stats.binary, stats.too_large

    existing = {r["path"]: r for r in store.q(
        "SELECT id, path, content_sha FROM files WHERE repo_id = ?", (repo_id,))}
    shas = {path: content_sha(data) for path, data in contents.items()}
    report.incremental = not full and bool(existing)

    added = [p for p in shas if p not in existing]
    modified = [p for p in shas if p in existing and existing[p]["content_sha"] != shas[p]]
    deleted = [p for p in existing if p not in shas]
    report.unchanged = len(shas) - len(added) - len(modified)

    # Renames: a deleted path whose exact content reappears under a new path.
    by_sha_added: dict[str, list[str]] = {}
    for p in added:
        by_sha_added.setdefault(shas[p], []).append(p)
    for old in list(deleted):
        candidates = by_sha_added.get(existing[old]["content_sha"])
        if candidates:
            new = candidates.pop(0)
            _rename_file(store, repo_id, existing[old]["id"], old, new, head)
            deleted.remove(old)
            added.remove(new)
            report.renamed.append((old, new))

    for p in deleted:
        file_id = existing[p]["id"]
        ids = [r[0] for r in store.q("SELECT id FROM symbols WHERE file_id = ?", (file_id,))]
        report.symbols_removed += len(ids)
        store._fts_delete(ids)
        store.db.execute("DELETE FROM files WHERE id = ?", (file_id,))
    report.deleted = deleted

    for p in sorted(added + modified):
        n_new, n_removed, lang = _index_file(store, repo_id, existing.get(p), p, contents[p], shas[p],
                                             head, embedder)
        report.symbols_indexed += n_new
        report.symbols_removed += n_removed
        report.languages[lang] += 1
    report.added, report.modified = sorted(added), sorted(modified)

    store.db.execute("UPDATE repos SET commit_sha = ?, embedder = ?, indexed_at = ? WHERE id = ?",
                     (head, embedder.name, time.time(), repo_id))
    store.commit()
    report.symbols_total = store.q1("SELECT COUNT(*) FROM symbols WHERE repo_id = ?", (repo_id,))[0]
    report.seconds = time.perf_counter() - t0

    if consolidate and (report.changed or full):
        from .memory import consolidate as run_consolidation
        n = run_consolidation(store, name)
        if n:
            report.notes.append(f"memory consolidated: {n} facts")
    return report


def _rename_file(store: Store, repo_id: int, file_id: int, old: str, new: str, head):
    store.db.execute("UPDATE files SET path = ?, commit_sha = ? WHERE id = ?", (new, head, file_id))
    prefix_old, prefix_new = f"{old}::", f"{new}::"
    store.db.execute(
        "UPDATE symbols SET key = ? || substr(key, ?) WHERE file_id = ?",
        (prefix_new, len(prefix_old) + 1, file_id))
    for table in ("wiki_citations", "feedback"):
        store.db.execute(
            f"UPDATE {table} SET symbol_key = ? || substr(symbol_key, ?) WHERE symbol_key LIKE ? ESCAPE '\\'",
            (prefix_new, len(prefix_old) + 1, _like_prefix(prefix_old)))


def _like_prefix(s: str) -> str:
    return s.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_") + "%"


def _index_file(store: Store, repo_id: int, existing_row, path: str, data: bytes, sha: str,
                head, embedder) -> tuple[int, int, str]:
    text = data.decode("utf-8", errors="replace")
    language = detect(path, data[:200])
    parsed = parse(path, text, language)
    now = time.time()
    if existing_row is None:
        cur = store.db.execute(
            "INSERT INTO files (repo_id, path, language, content_sha, size, lines, parser, module_doc,"
            " commit_sha, indexed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
            (repo_id, path, language, sha, len(data), text.count("\n") + 1, parsed.parser,
             parsed.module_doc, head, now))
        file_id = cur.lastrowid
    else:
        file_id = existing_row["id"]
        store.db.execute(
            "UPDATE files SET language = ?, content_sha = ?, size = ?, lines = ?, parser = ?,"
            " module_doc = ?, commit_sha = ?, indexed_at = ? WHERE id = ?",
            (language, sha, len(data), text.count("\n") + 1, parsed.parser, parsed.module_doc,
             head, now, file_id))

    old = {r["key"]: r for r in store.q("SELECT id, key, content_hash FROM symbols WHERE file_id = ?",
                                        (file_id,))}
    seen: Counter = Counter()
    keyed = []
    for sym in parsed.symbols:
        base = f"{path}::{sym.qualname}"
        seen[base] += 1
        key = base if seen[base] == 1 else f"{base}#{seen[base]}"
        keyed.append((key, sym, _symbol_hash(sym)))

    to_embed = [(key, sym, h) for key, sym, h in keyed
                if key not in old or old[key]["content_hash"] != h]
    vectors = embedder.embed([symbol_text(s.name, s.qualname, s.kind, s.signature, s.doc, s.source, path)
                              for _, s, _ in to_embed])
    vec_of = {key: vectors[i] for i, (key, _, _) in enumerate(to_embed)}

    for key, sym, h in keyed:
        if key in old and old[key]["content_hash"] == h:
            store.db.execute("UPDATE symbols SET start_line = ?, end_line = ? WHERE id = ?",
                             (sym.start_line, sym.end_line, old[key]["id"]))
            continue
        fields = (sym.name, sym.qualname, sym.kind, language, sym.start_line, sym.end_line,
                  sym.signature, sym.doc, sym.source, h, head, to_blob(vec_of[key]))
        if key in old:
            sid = old[key]["id"]
            store.db.execute(
                "UPDATE symbols SET name = ?, qualname = ?, kind = ?, language = ?, start_line = ?,"
                " end_line = ?, signature = ?, doc = ?, source = ?, content_hash = ?, commit_sha = ?,"
                " embedding = ? WHERE id = ?", fields + (sid,))
            store.db.execute("DELETE FROM calls WHERE src_id = ?", (sid,))
        else:
            cur = store.db.execute(
                "INSERT INTO symbols (repo_id, file_id, key, name, qualname, kind, language, start_line,"
                " end_line, signature, doc, source, content_hash, commit_sha, embedding)"
                " VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", (repo_id, file_id, key) + fields)
            sid = cur.lastrowid
        words = " ".join(split_identifier(sym.qualname))
        store.fts_put(sid, sym.name, sym.qualname, words, sym.doc, sym.source)
        store.db.executemany("INSERT OR IGNORE INTO calls (src_id, dst_name, repo_id) VALUES (?, ?, ?)",
                             [(sid, c, repo_id) for c in sym.calls])

    stale_keys = set(old) - {k for k, _, _ in keyed}
    stale_ids = [old[k]["id"] for k in stale_keys]
    store._fts_delete(stale_ids)
    store.db.executemany("DELETE FROM symbols WHERE id = ?", [(i,) for i in stale_ids])

    store.db.execute("DELETE FROM imports WHERE file_id = ?", (file_id,))
    store.db.executemany("INSERT OR IGNORE INTO imports (file_id, repo_id, module) VALUES (?, ?, ?)",
                         [(file_id, repo_id, m) for m in parsed.imports])
    return len(to_embed), len(stale_ids), language
