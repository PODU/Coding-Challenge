"""The ingestion graph.

    load_source -> extract_entities -> summarize -> upsert_pages
                -> update_topics -> detect_contradictions -> embed_changed -> log

Pages are only re-embedded and saved when their content hash changes, so
ingesting the same source twice does nothing the second time.
"""

from __future__ import annotations

from .agent import Agent, Entity
from .domain import (
    Contradiction,
    Frontmatter,
    IngestResult,
    Operation,
    WikiPage,
    utcnow_iso,
)
from .graph import Graph
from .interfaces import Embedder, Store
from .text import content_tokens, sha256_hex, slug


def _bullets(claims: list[str]) -> str:
    seen: list[str] = []
    for c in claims:
        c = c.strip()
        if c and c not in seen:
            seen.append(c)
    return "\n".join(f"- {c}" for c in seen)


class IngestionPipeline:
    def __init__(self, store: Store, embedder: Embedder, agent: Agent) -> None:
        self.store = store
        self.embedder = embedder
        self.agent = agent

    def run(
        self, project: str, *, text: str, title: str, path: str | None
    ) -> tuple[IngestResult, list[WikiPage], list[Operation]]:
        state: dict = {
            "project": project,
            "text": text,
            "title": title,
            "path": path,
            "result": IngestResult(),
            "changed": {},  # page_id -> WikiPage touched in this run
            "existing_before": set(),
            "entity_pages": [],  # input for the contradiction pass
            "ops": [],
            "skip": False,
        }
        self._build().run(state)
        return state["result"], list(state["changed"].values()), state["ops"]

    def _build(self) -> Graph:
        g = Graph()
        g.node("load_source", self._load_source)
        g.node("extract_entities", self._extract_entities)
        g.node("summarize", self._summarize)
        g.node("upsert_pages", self._upsert_pages)
        g.node("update_topics", self._update_topics)
        g.node("detect_contradictions", self._detect_contradictions)
        g.node("embed_changed", self._embed_changed)
        g.node("log", self._log)
        g.set_entry("load_source")
        g.conditional("load_source", lambda s: None if s["skip"] else "extract_entities")
        g.edge("extract_entities", "summarize")
        g.edge("summarize", "upsert_pages")
        g.edge("upsert_pages", "update_topics")
        g.edge("update_topics", "detect_contradictions")
        g.edge("detect_contradictions", "embed_changed")
        g.edge("embed_changed", "log")
        g.edge("log", None)
        return g

    def _load(self, project: str, page_id: str, state: dict) -> WikiPage | None:
        if page_id in state["changed"]:
            return state["changed"][page_id]
        existing = self.store.get_page(project, page_id)
        if existing is not None:
            state["existing_before"].add(page_id)
        return existing

    @staticmethod
    def _add_link(fm: Frontmatter, target: str) -> None:
        if target != fm.id and target not in fm.links:
            fm.links = sorted(set(fm.links + [target]))

    def _load_source(self, state: dict) -> None:
        project, text, path = state["project"], state["text"], state["path"]
        result: IngestResult = state["result"]
        if not text or not text.strip():
            op = Operation(op="skip_empty_source", details={"path": path})
            self.store.append_log(project, op)
            state["ops"].append(op)
            result.skipped.append(path or "<text>")
            state["skip"] = True
            return
        sha = sha256_hex(text.strip())
        existing = self.store.get_source_by_sha(project, sha)
        if existing is not None:
            result.skipped.append(existing)
            state["skip"] = True
            return
        source_id = "src_" + (slug(path) if path else sha[:12])
        self.store.add_source(project, source_id, state["title"], sha, path)
        state["source_id"] = source_id
        state["sha"] = sha
        result.source_ids.append(source_id)
        op = Operation(op="ingest_source", page_id=None, details={"source_id": source_id, "sha": sha})
        self.store.append_log(project, op)
        state["ops"].append(op)

    def _extract_entities(self, state: dict) -> None:
        state["entities"] = self.agent.extract_entities(state["text"])

    def _summarize(self, state: dict) -> None:
        names = [e.name for e in state["entities"]]
        state["summary"] = self.agent.summarize(state["text"], names)

    def _upsert_pages(self, state: dict) -> None:
        project = state["project"]
        source_id = state["source_id"]
        summary = state["summary"]
        title = state["title"] or summary.get("title", "Untitled")

        # One summary page per source.
        summary_id = f"summaries/{slug(title)}"
        spage = self._load(project, summary_id, state)
        if spage is None:
            spage = WikiPage(
                frontmatter=Frontmatter(
                    id=summary_id, type="summary", title=title, project=project
                ),
                body="",
            )
        spage.body = summary.get("summary", "").strip()
        if source_id not in spage.frontmatter.sources:
            spage.frontmatter.sources = sorted(set(spage.frontmatter.sources + [source_id]))
        spage.frontmatter.tags = sorted(set(spage.frontmatter.tags + summary.get("tags", [])))
        spage.frontmatter.updated_at = utcnow_iso()
        state["changed"][summary_id] = spage
        state["summary_id"] = summary_id

        entity_ids: list[str] = []
        for ent in state["entities"]:
            eid = f"entities/{slug(ent.name)}"
            entity_ids.append(eid)
            epage = self._load(project, eid, state)
            prior_claims: list[str] = []
            if epage is None:
                epage = WikiPage(
                    frontmatter=Frontmatter(
                        id=eid, type="entity", title=ent.name, project=project,
                        tags=[ent.type],
                    ),
                    body="",
                )
            else:
                prior_claims = [
                    ln[2:].strip() for ln in epage.body.splitlines() if ln.startswith("- ")
                ]
            merged = prior_claims + list(ent.salient_claims)
            epage.body = f"# {ent.name}\n\n{_bullets(merged)}"
            if source_id not in epage.frontmatter.sources:
                epage.frontmatter.sources = sorted(set(epage.frontmatter.sources + [source_id]))
            self._add_link(epage.frontmatter, summary_id)
            epage.frontmatter.updated_at = utcnow_iso()
            state["changed"][eid] = epage
            self._add_link(spage.frontmatter, eid)
            state["entity_pages"].append(
                {"id": eid, "name": ent.name, "new_claims": list(ent.salient_claims),
                 "prior_claims": prior_claims}
            )
        state["entity_ids"] = entity_ids

    def _update_topics(self, state: dict) -> None:
        project = state["project"]
        summary_id = state["summary_id"]
        spage = state["changed"][summary_id]
        for tag in state["summary"].get("tags", []):
            tid = f"topics/{slug(tag)}"
            tpage = self._load(project, tid, state)
            if tpage is None:
                tpage = WikiPage(
                    frontmatter=Frontmatter(
                        id=tid, type="topic", title=tag.title(), project=project, tags=[tag]
                    ),
                    body=f"# {tag.title()}\n\nRoll-up of pages tagged '{tag}'.",
                )
            self._add_link(tpage.frontmatter, summary_id)
            for eid in state["entity_ids"]:
                self._add_link(tpage.frontmatter, eid)
            tpage.frontmatter.updated_at = utcnow_iso()
            state["changed"][tid] = tpage
            self._add_link(spage.frontmatter, tid)

    def _detect_contradictions(self, state: dict) -> None:
        project = state["project"]
        summary_id = state["summary_id"]
        result: IngestResult = state["result"]
        for ent in state["entity_pages"]:
            for new_claim in ent["new_claims"]:
                for prior in ent["prior_claims"]:
                    if new_claim == prior:
                        continue
                    if self.agent.judge(new_claim, prior) == "contradict":
                        cid = sha256_hex("|".join(sorted([new_claim, prior]) + [ent["name"]]))[:16]
                        c = Contradiction(
                            id=cid, entity=ent["name"],
                            page_a=summary_id, claim_a=new_claim,
                            page_b=ent["id"], claim_b=prior,
                        )
                        self.store.add_contradiction(project, c)
                        result.contradictions.append(c)
                        op = Operation(op="contradiction", page_id=ent["id"],
                                       details={"entity": ent["name"], "id": cid})
                        self.store.append_log(project, op)
                        state["ops"].append(op)

    def _embed_changed(self, state: dict) -> None:
        project = state["project"]
        result: IngestResult = state["result"]
        to_embed: list[WikiPage] = []
        for page in state["changed"].values():
            new_sha = sha256_hex(page.body.strip())
            old = self.store.get_page(project, page.id)
            if old is not None and old.frontmatter.content_sha256 == new_sha and \
                    old.frontmatter.links == page.frontmatter.links:
                continue
            page.frontmatter.content_sha256 = new_sha
            to_embed.append(page)
        if not to_embed:
            return
        vecs = self.embedder.embed([p.embedding_text() for p in to_embed])
        for page, vec in zip(to_embed, vecs):
            created = page.id not in state["existing_before"]
            self.store.upsert_page(project, page, vec)
            (result.pages_created if created else result.pages_updated).append(page.id)
            op = Operation(op="upsert_page", page_id=page.id,
                           details={"created": created, "type": page.type})
            self.store.append_log(project, op)
            state["ops"].append(op)

    def _log(self, state: dict) -> None:
        state["result"].ops = len(state["ops"])
