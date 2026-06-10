"""Health checks for a project's pages.

``lint`` reports orphan pages, missing pages, broken links, stale pages, data
gaps and recorded contradictions. ``fix`` creates stubs for missing pages and
marks stale ones; running it twice is harmless.
"""

from __future__ import annotations

import re
from datetime import datetime, timedelta, timezone

from .domain import Finding, Frontmatter, Operation, WikiPage, utcnow_iso
from .interfaces import Embedder, Store

_WIKILINK_RE = re.compile(r"\[\[([^\]]+)\]\]")

# Topics are entry points and answers are leaves; nothing is expected to link
# to them.
_ORPHAN_EXEMPT_TYPES = {"topic", "answer"}
STALE_DAYS = 365


def _wikilinks(body: str) -> list[str]:
    return [m.group(1).strip() for m in _WIKILINK_RE.finditer(body)]


def lint(store: Store, project: str, *, stale_days: int = STALE_DAYS) -> list[Finding]:
    pages = store.all_pages(project)
    by_id = {p.id: p for p in pages}
    findings: list[Finding] = []

    inbound: dict[str, set[str]] = {p.id: set() for p in pages}
    for p in pages:
        for t in set(p.frontmatter.links) | set(_wikilinks(p.body)):
            inbound.setdefault(t, set()).add(p.id)

    # A link to a page that was deleted is "broken"; one that never existed is
    # "missing".
    deleted_ids = {
        op.page_id
        for op in store.read_log(project)
        if op.op == "delete_page" and op.page_id
    }

    for p in pages:
        if p.type not in _ORPHAN_EXEMPT_TYPES and not inbound.get(p.id):
            findings.append(Finding(
                kind="orphan", page_id=p.id, severity="warning",
                message=f"Page '{p.id}' has no inbound links.",
                suggestion="Link to it from a related topic or summary page.",
            ))
        if _is_stale(p, stale_days):
            findings.append(Finding(
                kind="stale", page_id=p.id, severity="info",
                message=f"Page '{p.id}' is marked or appears stale.",
                suggestion="Re-ingest the latest source or refresh the claims.",
            ))
        for target in sorted(set(p.frontmatter.links) | set(_wikilinks(p.body))):
            if target in by_id:
                continue
            if target in deleted_ids:
                findings.append(Finding(
                    kind="broken_link", page_id=p.id, target=target, severity="error",
                    message=f"Page '{p.id}' links to deleted page '{target}'.",
                    suggestion="Remove the link or recreate the page.",
                ))
            else:
                findings.append(Finding(
                    kind="missing_page", page_id=p.id, target=target, severity="warning",
                    message=f"Page '{p.id}' links to nonexistent page '{target}'.",
                    suggestion=f"Create a stub for '{target}'.",
                ))

    # Data gaps: summaries linking to entity pages that don't exist.
    entity_ids = {p.id for p in pages if p.type == "entity"}
    for p in pages:
        if p.type != "summary":
            continue
        for ent in p.frontmatter.links:
            if ent.startswith("entities/") and ent not in entity_ids and ent not in by_id:
                findings.append(Finding(
                    kind="data_gap", page_id=p.id, target=ent, severity="info",
                    message=f"Entity '{ent}' is referenced but has no page.",
                    suggestion=f"Create an entity page for '{ent}'.",
                ))

    for c in store.list_contradictions(project):
        findings.append(Finding(
            kind="contradiction", page_id=c.page_a, target=c.page_b, severity="error",
            message=(f"Contradiction about '{c.entity}': "
                     f"\"{c.claim_a}\" vs \"{c.claim_b}\"."),
            suggestion="Review both sources and reconcile or annotate.",
        ))

    findings.sort(key=lambda f: (f.kind, f.page_id or "", f.target or ""))
    return findings


def _is_stale(page: WikiPage, stale_days: int) -> bool:
    if page.frontmatter.status == "stale":
        return True
    try:
        updated = datetime.fromisoformat(page.frontmatter.updated_at)
    except ValueError:
        return False
    if updated.tzinfo is None:
        updated = updated.replace(tzinfo=timezone.utc)
    return updated < datetime.now(timezone.utc) - timedelta(days=stale_days)


def fix(store: Store, embedder: Embedder, project: str, findings: list[Finding]) -> list[str]:
    """Apply the fixes it can and return a description of each."""
    actions: list[str] = []
    for f in findings:
        if f.kind == "missing_page" and f.target:
            if store.get_page(project, f.target) is None:
                stub = _stub(project, f.target)
                vec = embedder.embed([stub.embedding_text()])[0]
                store.upsert_page(project, stub, vec)
                store.append_log(project, Operation(
                    op="upsert_page", page_id=f.target, details={"created": True, "stub": True}))
                actions.append(f"created stub {f.target}")
        elif f.kind == "stale" and f.page_id:
            page = store.get_page(project, f.page_id)
            if page and page.frontmatter.status != "stale":
                page.frontmatter.status = "stale"
                page.frontmatter.updated_at = utcnow_iso()
                vec = embedder.embed([page.embedding_text()])[0]
                store.upsert_page(project, page, vec)
                actions.append(f"marked stale {f.page_id}")
    return actions


def _stub(project: str, page_id: str) -> WikiPage:
    ptype = page_id.split("/", 1)[0].rstrip("s") if "/" in page_id else "topic"
    if ptype not in {"summary", "entity", "topic", "answer"}:
        ptype = "entity"
    title = page_id.split("/", 1)[-1].replace("-", " ").title()
    fm = Frontmatter(id=page_id, type=ptype, title=title, project=project, status="draft")
    page = WikiPage(frontmatter=fm, body=f"# {title}\n\n_Stub page created by linter --fix._")
    page.recompute_sha()
    return page
