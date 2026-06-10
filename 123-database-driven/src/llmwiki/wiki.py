"""The main entry point.

    wiki = Wiki(store=SqliteStore(":memory:"),
                embedder=HashingEmbedder(),
                llm=ScriptedLLM(),
                projects_root=tmp)
    wiki.create_project("p")
    wiki.ingest("p", text_or_path)   -> IngestResult
    wiki.ask("p", "question")        -> Answer
    wiki.lint("p")                   -> list[Finding]

Searches go to the store. The Markdown files, ``index.md`` and ``log.jsonl``
on disk are for people to browse and are rewritten after every change.
"""

from __future__ import annotations

import json
from pathlib import Path

from .agent import Agent
from .domain import (
    Answer,
    EmptyDocumentError,
    Finding,
    IngestResult,
    Operation,
    WikiPage,
)
from .frontmatter import to_markdown
from .ingestion import IngestionPipeline
from .interfaces import Embedder, LLMProvider, Store
from .linter import fix as _lint_fix
from .linter import lint as _lint
from .projects import ProjectManager
from .query import QueryPipeline


class ProjectNotFound(KeyError):
    pass


class Wiki:
    def __init__(
        self,
        store: Store,
        embedder: Embedder,
        llm: LLMProvider,
        projects_root: str | Path = "projects",
    ) -> None:
        self.store = store
        self.embedder = embedder
        self.llm = llm
        self.agent = Agent(llm)
        self.projects_root = Path(projects_root)
        self.pm = ProjectManager(store, self.projects_root)
        self._ingest = IngestionPipeline(store, embedder, self.agent)
        self._query = QueryPipeline(store, embedder, self.agent)

    def create_project(self, project: str) -> None:
        self.pm.create(project)
        self._write_index(project)

    def list_projects(self) -> list[str]:
        return self.pm.list()

    def delete_project(self, project: str) -> None:
        self.pm.delete(project)

    def _require(self, project: str) -> None:
        if not self.pm.exists(project):
            raise ProjectNotFound(project)

    def ingest(self, project: str, source: str | Path, *, title: str | None = None) -> IngestResult:
        self._require(project)
        agg = IngestResult()
        for text, path, derived_title in self._resolve_sources(source):
            res, pages, ops = self._ingest.run(
                project, text=text, title=title or derived_title, path=path
            )
            self._persist(project, text, path, res, pages, ops)
            self._merge_result(agg, res)
        self._write_index(project)
        return agg

    def ingest_text(self, project: str, text: str, *, title: str = "document") -> IngestResult:
        self._require(project)
        if not text or not text.strip():
            raise EmptyDocumentError("refusing to ingest empty/whitespace-only document")
        res, pages, ops = self._ingest.run(project, text=text, title=title, path=None)
        self._persist(project, text, None, res, pages, ops)
        self._write_index(project)
        return res

    @staticmethod
    def _resolve_sources(source: str | Path) -> list[tuple[str, str | None, str]]:
        p = Path(source)
        # A string that isn't an existing path is treated as the text itself.
        if isinstance(source, Path) or (isinstance(source, str) and p.exists()):
            if p.is_dir():
                files = sorted(f for f in p.rglob("*") if f.is_file())
                return [(f.read_text(encoding="utf-8"), str(f), f.stem) for f in files]
            return [(p.read_text(encoding="utf-8"), str(p), p.stem)]
        return [(str(source), None, "document")]

    @staticmethod
    def _merge_result(agg: IngestResult, res: IngestResult) -> None:
        agg.source_ids += res.source_ids
        agg.pages_created += res.pages_created
        agg.pages_updated += res.pages_updated
        agg.contradictions += res.contradictions
        agg.skipped += res.skipped
        agg.ops += res.ops

    def _persist(
        self, project: str, text: str, path: str | None,
        res: IngestResult, pages: list[WikiPage], ops: list[Operation],
    ) -> None:
        base = self.pm.project_dir(project)
        if res.source_ids:
            (base / "raw").mkdir(parents=True, exist_ok=True)
            (base / "raw" / f"{res.source_ids[0]}.txt").write_text(text, encoding="utf-8")
        for page in pages:
            if page.id in res.pages_created or page.id in res.pages_updated:
                self._write_page_file(project, page)
        self._append_log_file(project, ops)

    def _write_page_file(self, project: str, page: WikiPage) -> None:
        fp = self.pm.project_dir(project) / "wiki" / f"{page.id}.md"
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(to_markdown(page), encoding="utf-8")

    def _append_log_file(self, project: str, ops: list[Operation]) -> None:
        if not ops:
            return
        fp = self.pm.project_dir(project) / "log.jsonl"
        fp.parent.mkdir(parents=True, exist_ok=True)
        with fp.open("a", encoding="utf-8") as fh:
            for op in ops:
                fh.write(json.dumps(op.to_dict(), ensure_ascii=False) + "\n")

    def ask(self, project: str, question: str, *, k: int = 6, save: bool = False) -> Answer:
        self._require(project)
        answer = self._query.run(project, question, k=k, save=save)
        if answer.saved_as:
            page = self.store.get_page(project, answer.saved_as)
            if page:
                self._write_page_file(project, page)
            self._append_log_file(project, [Operation(op="save_answer", page_id=answer.saved_as)])
            self._write_index(project)
        return answer

    def lint(self, project: str, *, stale_days: int = 365) -> list[Finding]:
        self._require(project)
        return _lint(self.store, project, stale_days=stale_days)

    def lint_fix(self, project: str) -> list[str]:
        self._require(project)
        findings = _lint(self.store, project)
        actions = _lint_fix(self.store, self.embedder, project, findings)
        if actions:
            self._write_index(project)
            for f in findings:
                if f.kind == "missing_page" and f.target:
                    page = self.store.get_page(project, f.target)
                    if page:
                        self._write_page_file(project, page)
        return actions

    def get_page(self, project: str, page_id: str) -> WikiPage | None:
        self._require(project)
        return self.store.get_page(project, page_id)

    def browse(self, project: str, *, type: str | None = None) -> list[WikiPage]:
        self._require(project)
        pages = self.store.all_pages(project)
        if type:
            pages = [p for p in pages if p.type == type]
        return pages

    def read_log(self, project: str) -> list[Operation]:
        self._require(project)
        return self.store.read_log(project)

    def search(self, project: str, query: str, *, k: int = 6):
        self._require(project)
        qvec = self.embedder.embed([query])[0]
        return self.store.hybrid_search(project, query, qvec, k)

    def _write_index(self, project: str) -> None:
        pages = self.store.all_pages(project)
        lines = [f"# Wiki Index — {project}", ""]
        for ptype, heading in (
            ("topic", "Topics"), ("summary", "Summaries"),
            ("entity", "Entities"), ("answer", "Answers"),
        ):
            group = [p for p in pages if p.type == ptype]
            if not group:
                continue
            lines.append(f"## {heading}")
            lines.extend(f"- [[{p.id}]] — {p.frontmatter.title}" for p in group)
            lines.append("")
        lines.append(f"_{len(pages)} pages._")
        fp = self.pm.project_dir(project) / "index.md"
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text("\n".join(lines) + "\n", encoding="utf-8")
