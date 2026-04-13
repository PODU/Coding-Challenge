"""The Sherpa orchestrator.

``ingest`` re-indexes only the files that changed since the last run and drops
files that were deleted. ``ask`` retrieves relevant chunks, builds a prompt
with memory, calls the LLM and returns an answer with citations.

Every component is passed in so tests can use mocks; ``from_config`` builds
the real ones.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass

from ..config import SherpaConfig
from ..embed.base import Embedder
from ..ingest.chunker import CodeChunker
from ..ingest.walker import RepoWalker
from ..llm.base import LLMClient, LLMError, Message
from ..memory.base import MemoryStore
from ..memory.in_memory import make_episode
from ..models import Answer, CodeChunk
from ..project.registry import Changes, ProjectRegistry
from ..prompt.builder import PromptBuilder
from ..prompt.select import ContextSelector
from ..retrieve.classify import QueryClassifier
from ..retrieve.hybrid import HybridRetriever
from ..store.base import VectorStore

NOT_FOUND_SENTENCE = "I couldn't find that in this codebase."

ProgressFn = Callable[[str], None]


@dataclass
class IngestReport:
    project: str
    files_seen: int
    files_ingested: int
    files_unchanged: int
    files_removed: int
    chunks_indexed: int
    total_chunks: int

    @property
    def changed_anything(self) -> bool:
        return bool(self.files_ingested or self.files_removed)


class Sherpa:
    def __init__(
        self,
        *,
        walker: RepoWalker,
        chunker: CodeChunker,
        embedder: Embedder,
        store: VectorStore,
        retriever: HybridRetriever,
        selector: ContextSelector,
        builder: PromptBuilder,
        llm: LLMClient,
        memory: MemoryStore,
        registry: ProjectRegistry,
        classifier: QueryClassifier,
        config: SherpaConfig,
    ) -> None:
        self.walker = walker
        self.chunker = chunker
        self.embedder = embedder
        self.store = store
        self.retriever = retriever
        self.selector = selector
        self.builder = builder
        self.llm = llm
        self.memory = memory
        self.registry = registry
        self.classifier = classifier
        self.config = config

    @classmethod
    def from_config(cls, config: SherpaConfig, registry_path: str | None = None) -> "Sherpa":
        from ..embed import build_embedder
        from ..llm import build_llm
        from ..memory import build_memory
        from ..store import build_store

        store = build_store(config)
        embedder = build_embedder(config)
        return cls(
            walker=RepoWalker(max_file_bytes=config.max_file_bytes),
            chunker=CodeChunker(
                max_chunk_chars=config.max_chunk_chars,
                min_chunk_chars=config.min_chunk_chars,
                window_size=config.window_size,
                window_overlap=config.window_overlap,
            ),
            embedder=embedder,
            store=store,
            retriever=HybridRetriever(
                store=store,
                embedder=embedder,
                rrf_k=config.rrf_k,
                vector_weight=config.vector_weight,
                text_weight=config.text_weight,
                min_vector_score=config.min_vector_score,
            ),
            selector=ContextSelector(char_budget=config.context_char_budget),
            builder=PromptBuilder(),
            llm=build_llm(config),
            memory=build_memory(config),
            registry=ProjectRegistry(registry_path),
            classifier=QueryClassifier(
                base_top_k=config.top_k,
                vector_weight=config.vector_weight,
                text_weight=config.text_weight,
            ),
            config=config,
        )

    def ingest(
        self,
        project: str,
        path: str,
        *,
        progress: ProgressFn | None = None,
        force: bool = False,
    ) -> IngestReport:
        self.registry.register(project, path)
        records = list(self.walker.walk(path))
        changes = self.registry.diff(project, records)
        if force:
            # Ignore stored hashes and index everything. Used to refill an
            # in-memory store, which starts empty in every process.
            changes = Changes(
                added=changes.added + changes.changed + changes.unchanged,
                changed=[],
                removed=changes.removed,
                unchanged=[],
            )

        for removed_path in changes.removed:
            self.store.delete_by_file(project, removed_path)
            self._emit(progress, f"removed {removed_path}")

        chunks_indexed = 0
        for rec in changes.to_ingest:
            self.store.delete_by_file(project, rec.path)
            chunks = self.chunker.chunk_file(project, rec)
            if chunks:
                self._embed_and_store(chunks)
                chunks_indexed += len(chunks)
            self._emit(progress, f"indexed {rec.path} ({len(chunks)} chunks)")

        total = self.store.count(project)
        self.registry.commit(
            project, changes.to_ingest, removed=changes.removed, chunk_count=total
        )
        return IngestReport(
            project=project,
            files_seen=len(records),
            files_ingested=len(changes.to_ingest),
            files_unchanged=len(changes.unchanged),
            files_removed=len(changes.removed),
            chunks_indexed=chunks_indexed,
            total_chunks=total,
        )

    def reingest(self, project: str, *, progress: ProgressFn | None = None) -> IngestReport:
        info = self.registry.get(project)
        if info is None or not info.root:
            raise KeyError(f"Unknown project {project!r}; ingest it first.")
        return self.ingest(project, info.root, progress=progress)

    def _embed_and_store(self, chunks: list[CodeChunk]) -> None:
        vectors = self.embedder.embed([c.code for c in chunks])
        self.store.upsert(chunks, vectors)

    def ask(self, project: str, question: str) -> Answer:
        if not question.strip():
            return Answer(
                text="Please provide a question.",
                insufficient_context=True,
                classification=None,
            )

        cls = self.classifier.classify(question)
        scored = self.retriever.retrieve(
            project,
            question,
            k=cls.top_k,
            vector_weight=cls.vector_weight,
            text_weight=cls.text_weight,
        )
        context = self.selector.select(scored)

        facts = self.memory.relevant_facts(project, question)
        episodes = self.memory.search_episodes(project, question)
        used_memory = bool(facts or episodes)

        # Nothing to ground an answer on, so don't bother the LLM.
        if context.insufficient_context and not used_memory:
            answer = Answer(
                text=NOT_FOUND_SENTENCE,
                citations=[],
                used_memory=False,
                insufficient_context=True,
                classification=cls.query_class.value,
            )
            self._record(project, question, answer)
            return answer

        messages = self.builder.build_messages(question, context, facts, episodes)
        try:
            resp = self.llm.complete(
                messages,
                model=self.config.model,
                max_tokens=self.config.max_tokens,
            )
            text = resp.text
            error = None
        except LLMError as exc:
            text = (
                "The language model call failed, but here are the most relevant code "
                "locations I found."
            )
            error = str(exc)

        answer = Answer(
            text=text,
            citations=context.citations,
            used_memory=used_memory,
            insufficient_context=context.insufficient_context,
            classification=cls.query_class.value,
            error=error,
        )
        self._record(project, question, answer)
        return answer

    def _record(self, project: str, question: str, answer: Answer) -> None:
        self.memory.add_episode(
            make_episode(
                project=project,
                question=question,
                answer=answer.text,
                citations=[c.label() for c in answer.citations],
            )
        )

    def add_fact(self, project: str, key: str, value: str) -> None:
        from ..memory.base import SemanticFact

        self.memory.add_fact(SemanticFact(project=project, key=key, value=value))

    @staticmethod
    def _emit(progress: ProgressFn | None, msg: str) -> None:
        if progress is not None:
            progress(msg)
