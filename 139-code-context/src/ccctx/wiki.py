"""Generated wiki: one page per module (directory) with >= 10 indexed symbols.

Pages are generated from the index alone (module docs, the most
depended-upon symbols, entry points called from other modules, and import
relationships), so rebuilding gives the same page every time. Every
symbol a page mentions is also stored as a row in `wiki_citations` with a
foreign key to `symbols` and the symbol's content hash at build time. A page
is stale when a cited symbol was deleted (FK set to NULL), its code changed
(hash mismatch), or files in the module were re-indexed after the page was
built. `wiki build` recompiles only stale or missing pages.
"""

from __future__ import annotations

import time
from collections import Counter, defaultdict
from dataclasses import dataclass, field

from .embed import get_embedder, to_blob
from .graph import file_dependencies, module_of
from .store import Store

MIN_SYMBOLS = 10
KEY_SYMBOLS = 10
STRIP_PREFIXES = ("src/", "lib/", "pkg/")


def slug_for(repo: str, module: str) -> str:
    m = module
    for p in STRIP_PREFIXES:
        if m.startswith(p):
            m = m[len(p):]
            break
    if m in ("src", "lib", "pkg"):
        m = ""
    return m or repo


@dataclass
class PageStatus:
    slug: str
    stale: bool
    reasons: list[str] = field(default_factory=list)


def _first_sentence(doc: str) -> str:
    doc = " ".join(doc.split())
    for end in (". ", ".\n"):
        if end in doc:
            return doc.split(end, 1)[0] + "."
    return doc[:160]


def _purpose(doc: str, limit: int = 500) -> str:
    """First real paragraph(s) of a module docstring, minus reST headings and directives."""
    paras = []
    for para in doc.split("\n\n"):
        lines = [l for l in para.strip().splitlines()
                 if l.strip() and not set(l.strip()) <= set("=-~^*#") and not l.strip().startswith("..")]
        if len(lines) == 1 and len(para.strip().splitlines()) == 2:
            continue  # a heading with its underline
        text = " ".join(" ".join(lines).split())
        if text:
            paras.append(text)
        if sum(len(p) for p in paras) > limit // 2:
            break
    out = " ".join(paras)
    return out if len(out) <= limit else out[:limit].rsplit(" ", 1)[0] + " ..."


def modules(store: Store, repo: str) -> dict[str, list]:
    r = store.repo(repo)
    rows = store.q("SELECT s.id, s.key, s.name, s.qualname, s.kind, s.doc, s.signature, s.content_hash,"
                   " s.start_line, f.path FROM symbols s JOIN files f ON f.id = s.file_id"
                   " WHERE s.repo_id = ? AND s.kind != 'chunk'", (r["id"],))
    by_mod: dict[str, list] = defaultdict(list)
    for row in rows:
        by_mod[module_of(row["path"])].append(row)
    return {m: syms for m, syms in by_mod.items() if len(syms) >= MIN_SYMBOLS}


def _compile(store: Store, repo: str, module: str, syms: list, deps: dict[str, set[str]],
             callers_by_name: dict[str, Counter]) -> tuple[str, str, list[tuple]]:
    r = store.repo(repo)
    files = sorted({s["path"] for s in syms})
    # Any file directly in the module counts, including symbol-less ones like __init__.py / doc.go.
    module_docs = [row[1] for row in store.q(
        "SELECT path, module_doc FROM files WHERE repo_id = ? AND module_doc != '' "
        "ORDER BY CASE WHEN path LIKE '%__init__.py' THEN 0 WHEN path LIKE '%doc.go' THEN 0 ELSE 1 END, path",
        (r["id"],)) if module_of(row[0]) == module]

    def score(s):
        ext = callers_by_name.get(s["name"], Counter())
        inbound = sum(ext.values())
        public = 0 if s["name"].startswith("_") else 1
        top_level = 1 if "." not in s["qualname"] else 0
        return (public, inbound + 3 * top_level + (2 if s["doc"] else 0), s["kind"] == "class")

    ranked = sorted(syms, key=lambda s: score(s), reverse=True)
    key_syms = ranked[:KEY_SYMBOLS]
    entry = []
    for s in syms:
        outside = {m for m in callers_by_name.get(s["name"], Counter()) if m != module}
        if outside and not s["name"].startswith("_") and s not in key_syms:
            entry.append((s, sorted(outside)))
    entry.sort(key=lambda e: (-len(e[1]), e[0]["qualname"]))
    entry = entry[:8]

    imports_out = sorted({module_of(t) for f in files for t in deps.get(f, ()) if module_of(t) != module})
    imported_by = sorted({module_of(src) for src, targets in deps.items() if module_of(src) != module
                          and any(module_of(t) == module for t in targets)})

    kinds = Counter(s["kind"] for s in syms)
    title = f"{slug_for(repo, module)}"
    out = [f"# {title}", "", f"Module `{module or '.'}` in repository **{repo}** - "
           f"{len(syms)} symbols in {len(files)} files "
           f"({', '.join(f'{n} {k}' for k, n in kinds.most_common())}).", "", "## Purpose", ""]
    if module_docs:
        out.append(_purpose(module_docs[0]))
    else:
        documented = [s for s in key_syms if s["doc"]][:3]
        if documented:
            out.append("Inferred from its key symbols: " + " ".join(
                f"`{s['qualname']}`: {_first_sentence(s['doc'])}" for s in documented))
        else:
            out.append("No module documentation; see the key symbols below.")
    out += ["", "## Key symbols", ""]
    citations = []
    for s in key_syms:
        desc = _first_sentence(s["doc"]) if s["doc"] else s["signature"][:100]
        out.append(f"- `{s['qualname']}` ({s['kind']}) - {desc} [{s['path']}:{s['start_line']}]")
        citations.append((s["id"], s["key"], "key", s["content_hash"]))
    if entry:
        out += ["", "## Entry points used by other modules", ""]
        for s, mods in entry:
            out.append(f"- `{s['qualname']}` - called from {', '.join(m or '.' for m in mods[:4])}")
            citations.append((s["id"], s["key"], "entry", s["content_hash"]))
    out += ["", "## Dependencies", ""]
    out.append("- Imports: " + (", ".join(f"`{m or '.'}`" for m in imports_out) or "(no in-repo imports)"))
    out.append("- Imported by: " + (", ".join(f"`{m or '.'}`" for m in imported_by) or "(nothing in-repo)"))
    out += ["", "## Files", ""] + [f"- {f}" for f in files]
    return title, "\n".join(out) + "\n", citations


def _embed_text(title: str, module: str, body: str, syms: list) -> str:
    """A compact summary to embed: what the module is called, its purpose, its key names."""
    name = title.rsplit("/", 1)[-1]
    purpose = body.split("## Purpose", 1)[-1].split("##", 1)[0]
    names = " ".join(sorted({s["name"] for s in syms if not s["name"].startswith("_")})[:40])
    return f"{name} {name} {name} module package {title} {module}\n{purpose[:600]}\n{names}"


def page_status(store: Store, page) -> PageStatus:
    reasons = []
    rows = store.q("SELECT c.symbol_key, c.content_hash, c.symbol_id, s.content_hash AS now_hash "
                   "FROM wiki_citations c LEFT JOIN symbols s ON s.id = c.symbol_id WHERE c.page_id = ?",
                   (page["id"],))
    for row in rows:
        if row["symbol_id"] is None:
            reasons.append(f"cited symbol removed: {row['symbol_key']}")
        elif row["now_hash"] != row["content_hash"]:
            reasons.append(f"cited symbol changed: {row['symbol_key']}")
    module = page["module_path"]
    like = f"{module}/%" if module else "%"
    changed = store.q1("SELECT COUNT(*) FROM files WHERE repo_id = ? AND path LIKE ? AND indexed_at > ? "
                       "AND (? = '' OR path NOT LIKE ?)",
                       (page["repo_id"], like, page["built_at"], module, f"{module}/%/%"))[0]
    if module == "":
        changed = store.q1("SELECT COUNT(*) FROM files WHERE repo_id = ? AND path NOT LIKE '%/%' "
                           "AND indexed_at > ?", (page["repo_id"], page["built_at"]))[0]
    if changed:
        reasons.append(f"{changed} file(s) in module re-indexed since build")
    return PageStatus(page["slug"], bool(reasons), reasons)


def build(store: Store, repo: str, force: bool = False, embedder=None) -> dict:
    r = store.repo(repo)
    if r is None:
        raise KeyError(f"unknown repository '{repo}'")
    embedder = embedder or get_embedder(r["embedder"])
    mods = modules(store, repo)
    deps = file_dependencies(store, repo)

    # Who calls each name, by caller module (for ranking and entry points).
    callers_by_name: dict[str, Counter] = defaultdict(Counter)
    for name, path in store.q("SELECT c.dst_name, f.path FROM calls c JOIN symbols s ON s.id = c.src_id "
                              "JOIN files f ON f.id = s.file_id WHERE c.repo_id = ?", (r["id"],)):
        callers_by_name[name][module_of(path)] += 1

    existing = {p["module_path"]: p for p in store.q("SELECT * FROM wiki_pages WHERE repo_id = ?", (r["id"],))}
    built, fresh, removed = [], [], []
    for module, syms in sorted(mods.items()):
        page = existing.get(module)
        if page is not None and not force and not page_status(store, page).stale \
                and page["commit_sha"] == r["commit_sha"]:
            fresh.append(page["slug"])
            continue
        if page is not None and not force and not page_status(store, page).stale:
            # Commit moved but nothing this page cites changed: re-stamp, don't recompile.
            store.db.execute("UPDATE wiki_pages SET commit_sha = ? WHERE id = ?", (r["commit_sha"], page["id"]))
            fresh.append(page["slug"])
            continue
        title, body, citations = _compile(store, repo, module, syms, deps, callers_by_name)
        slug = slug_for(repo, module)
        vec = to_blob(embedder.embed([f"{title}\n{body}"])[0])
        now = time.time()
        if page is None:
            cur = store.db.execute(
                "INSERT INTO wiki_pages (repo_id, slug, module_path, title, body, commit_sha, built_at, embedding)"
                " VALUES (?, ?, ?, ?, ?, ?, ?, ?)", (r["id"], slug, module, title, body, r["commit_sha"], now, vec))
            pid = cur.lastrowid
        else:
            pid = page["id"]
            store.db.execute("UPDATE wiki_pages SET slug = ?, title = ?, body = ?, commit_sha = ?, built_at = ?,"
                             " embedding = ? WHERE id = ?", (slug, title, body, r["commit_sha"], now, vec, pid))
            store.db.execute("DELETE FROM wiki_citations WHERE page_id = ?", (pid,))
        store.db.executemany("INSERT OR IGNORE INTO wiki_citations (page_id, symbol_id, symbol_key, role,"
                             " content_hash) VALUES (?, ?, ?, ?, ?)",
                             [(pid, sid, key, role, h) for sid, key, role, h in citations])
        store.db.execute("DELETE FROM wiki_fts WHERE rowid = ?", (pid,))
        store.db.execute("INSERT INTO wiki_fts (rowid, title, body) VALUES (?, ?, ?)", (pid, title, body))
        built.append(slug)
    for module, page in existing.items():
        if module not in mods:
            store.db.execute("DELETE FROM wiki_fts WHERE rowid = ?", (page["id"],))
            store.db.execute("DELETE FROM wiki_pages WHERE id = ?", (page["id"],))
            removed.append(page["slug"])
    store.commit()
    return {"built": built, "fresh": fresh, "removed": removed}


def find_page(store: Store, slug: str, repo: str | None = None):
    sql = "SELECT p.*, r.name AS repo_name FROM wiki_pages p JOIN repos r ON r.id = p.repo_id WHERE p.slug = ?"
    params: list = [slug]
    if repo:
        sql += " AND r.name = ?"
        params.append(repo)
    page = store.q1(sql, params)
    if page is None and "/" in slug and not repo:
        # Allow "<repo>/<slug>" as well as a bare slug.
        head, rest = slug.split("/", 1)
        page = store.q1(sql + " AND r.name = ?", [rest, head])
    return page


def list_pages(store: Store, repo: str | None = None) -> list[dict]:
    sql = "SELECT p.*, r.name AS repo_name FROM wiki_pages p JOIN repos r ON r.id = p.repo_id"
    params: list = []
    if repo:
        sql += " WHERE r.name = ?"
        params.append(repo)
    out = []
    for p in store.q(sql + " ORDER BY r.name, p.slug", params):
        st = page_status(store, p)
        out.append({"repo": p["repo_name"], "slug": p["slug"], "module": p["module_path"],
                    "stale": st.stale, "reasons": st.reasons})
    return out


def citations(store: Store, page) -> list[dict]:
    rows = store.q("SELECT c.symbol_key, c.role, c.content_hash, c.symbol_id, s.content_hash AS now_hash,"
                   " s.start_line, f.path FROM wiki_citations c LEFT JOIN symbols s ON s.id = c.symbol_id"
                   " LEFT JOIN files f ON f.id = s.file_id WHERE c.page_id = ? ORDER BY c.role DESC, c.symbol_key",
                   (page["id"],))
    out = []
    for row in rows:
        status = ("missing" if row["symbol_id"] is None
                  else "fresh" if row["now_hash"] == row["content_hash"] else "changed")
        out.append({"symbol_key": row["symbol_key"], "role": row["role"], "symbol_id": row["symbol_id"],
                    "status": status, "location": f"{row['path']}:{row['start_line']}" if row["path"] else None})
    return out


def rebuild_stale(store: Store, repo: str) -> dict | None:
    """Called after ingest: recompile stale pages if this repo has a wiki."""
    r = store.repo(repo)
    if not store.q1("SELECT 1 FROM wiki_pages WHERE repo_id = ? LIMIT 1", (r["id"],)):
        return None
    return build(store, repo)
