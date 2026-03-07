"""REST API: ``POST /ingest``, ``POST /ask``, ``GET /health``, ``GET /metrics``.

Needs the optional ``api`` extra (FastAPI). ``POST /ingest`` also accepts
documents inline, so the server doesn't need to share a filesystem with the
caller.
"""

# NB: no ``from __future__ import annotations`` here -- FastAPI/pydantic need the
# request-body classes resolvable as real types, not stringized forward refs.

from typing import Optional

from .config import Config
from .factory import build_agent


def create_app(
    *,
    use_redis: bool = False,
    mock_llm: bool = False,
    local_embedder: bool = False,
    cfg: Optional[Config] = None,
    agent=None,
):
    """Create the FastAPI app. FastAPI is imported here so the rest of the
    package works without it."""
    try:
        from fastapi import FastAPI, HTTPException
        from pydantic import BaseModel
    except Exception as exc:  # pragma: no cover - requires the 'api' extra
        raise RuntimeError(
            "The REST API requires FastAPI. Install with: pip install 'rtfm[api]'."
        ) from exc

    cfg = cfg or Config.from_env()
    agent = agent or build_agent(
        cfg, use_redis=use_redis, mock_llm=mock_llm, local_embedder=local_embedder
    )

    from .ingest import ingest_documents, ingest_paths

    class IngestRequest(BaseModel):
        paths: Optional[list[str]] = None
        documents: Optional[dict[str, str]] = None

    class AskRequest(BaseModel):
        question: str
        session_id: Optional[str] = None
        user_id: Optional[str] = None
        filename: Optional[str] = None

    app = FastAPI(title="RTFM For Me Agent", version="0.1.0")
    app.state.agent = agent

    @app.get("/health")
    def health() -> dict:
        return {"status": "ok", "indexed_chunks": agent.store.count(), "model": cfg.model}

    @app.get("/metrics")
    def metrics() -> dict:
        stats = agent.cache.stats if agent.cache is not None else None
        return {
            "indexed_chunks": agent.store.count(),
            "cache": (
                {
                    "hits": stats.hits,
                    "misses": stats.misses,
                    "hit_rate": stats.hit_rate,
                    "latency_saved_ms": stats.latency_saved_ms,
                    "cost_saved_usd": stats.cost_saved_usd,
                }
                if stats is not None
                else None
            ),
        }

    @app.post("/ingest")
    def ingest(req: IngestRequest) -> dict:
        if not req.paths and not req.documents:
            raise HTTPException(status_code=400, detail="provide 'paths' or 'documents'")
        chunks = 0
        failures: list = []
        if req.documents:
            report = ingest_documents(
                req.documents, embedder=agent.embedder, store=agent.store, cfg=cfg
            )
            chunks += report.chunks_added
            failures += report.files_failed
        if req.paths:
            report = ingest_paths(
                req.paths, embedder=agent.embedder, store=agent.store, cfg=cfg
            )
            chunks += report.chunks_added
            failures += report.files_failed
        return {"chunks_added": chunks, "failed": failures, "indexed_chunks": agent.store.count()}

    @app.post("/ask")
    def ask(req: AskRequest) -> dict:
        result = agent.ask(
            req.question,
            session_id=req.session_id,
            user_id=req.user_id,
            filename=req.filename,
        )
        ans = result.answer
        return {
            "answer": ans.text,
            "found": ans.found,
            "refused": ans.refused,
            "cached": result.cached,
            "degraded": result.degraded,
            "citations": [
                {"filename": c.filename, "section": c.section, "position": c.position}
                for c in ans.citations
            ],
        }

    return app
