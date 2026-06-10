"""The query graph.

    plan -> hybrid_retrieve -> synthesize -> attach_citations -> maybe_save_answer

Answers come only from retrieved pages. If nothing is retrieved the answer says
so and has no citations.
"""

from __future__ import annotations

from .agent import Agent
from .domain import Answer, Frontmatter, Operation, WikiPage, utcnow_iso
from .graph import Graph
from .interfaces import Embedder, Store
from .text import slug


class QueryPipeline:
    def __init__(self, store: Store, embedder: Embedder, agent: Agent) -> None:
        self.store = store
        self.embedder = embedder
        self.agent = agent

    def run(self, project: str, question: str, *, k: int = 6, save: bool = False) -> Answer:
        state: dict = {
            "project": project,
            "question": question,
            "k": k,
            "save": save,
            "answer": Answer(project=project, question=question, answer=""),
        }
        self._build().run(state)
        return state["answer"]

    def _build(self) -> Graph:
        g = Graph()
        g.node("plan", self._plan)
        g.node("hybrid_retrieve", self._hybrid_retrieve)
        g.node("synthesize", self._synthesize)
        g.node("attach_citations", self._attach_citations)
        g.node("maybe_save_answer", self._maybe_save_answer)
        g.set_entry("plan")
        g.edge("plan", "hybrid_retrieve")
        g.edge("hybrid_retrieve", "synthesize")
        g.edge("synthesize", "attach_citations")
        g.edge("attach_citations", "maybe_save_answer")
        g.edge("maybe_save_answer", None)
        return g

    def _plan(self, state: dict) -> None:
        state["query_vec"] = self.embedder.embed([state["question"]])[0]

    def _hybrid_retrieve(self, state: dict) -> None:
        hits = self.store.hybrid_search(
            state["project"], state["question"], state["query_vec"], state["k"]
        )
        pages: list[dict] = []
        retrieved: list[dict] = []
        for h in hits:
            page = self.store.get_page(state["project"], h.page_id)
            if page is None:
                continue
            pages.append({
                "page_id": page.id,
                "title": page.frontmatter.title,
                "body": page.body,
                "sources": page.frontmatter.sources,
            })
            retrieved.append({"page_id": h.page_id, "score": round(h.score, 6), "via": h.via})
        state["pages"] = pages
        state["answer"].retrieved = retrieved

    def _synthesize(self, state: dict) -> None:
        out = self.agent.synthesize(state["question"], state["pages"])
        state["answer"].answer = out.get("answer", "")
        state["raw_citations"] = out.get("citations", [])

    def _attach_citations(self, state: dict) -> None:
        state["answer"].citations = [
            {
                "marker": c.get("marker"),
                "page_id": c.get("page_id"),
                "source_id": c.get("source_id"),
                "quote": c.get("quote", ""),
            }
            for c in state["raw_citations"]
        ]

    def _maybe_save_answer(self, state: dict) -> None:
        answer: Answer = state["answer"]
        if not state["save"] or not answer.citations:
            return
        project = state["project"]
        aid = f"answers/{slug(state['question'])}"
        cited_pages = sorted({c["page_id"] for c in answer.citations if c.get("page_id")})
        cited_sources = sorted({c["source_id"] for c in answer.citations if c.get("source_id")})
        existing = self.store.get_page(project, aid)
        fm = existing.frontmatter if existing else Frontmatter(
            id=aid, type="answer", title=state["question"], project=project
        )
        fm.title = state["question"]
        fm.links = cited_pages
        fm.sources = cited_sources
        fm.tags = sorted(set(fm.tags + ["qa"]))
        fm.updated_at = utcnow_iso()
        body = f"**Q:** {state['question']}\n\n**A:** {answer.answer}"
        page = WikiPage(frontmatter=fm, body=body)
        page.recompute_sha()
        vec = self.embedder.embed([page.embedding_text()])[0]
        self.store.upsert_page(project, page, vec)
        self.store.append_log(project, Operation(op="save_answer", page_id=aid, details={}))
        answer.saved_as = aid
        state["saved_page"] = page
