"""REST API smoke tests (skipped if FastAPI/httpx test client are unavailable)."""

from __future__ import annotations

import pytest

pytest.importorskip("fastapi")
pytest.importorskip("httpx")

from fastapi.testclient import TestClient  # noqa: E402

from rtfm.agent import RTFMAgent  # noqa: E402
from rtfm.api import create_app  # noqa: E402
from rtfm.config import Config  # noqa: E402
from rtfm.embedder import HashEmbedder  # noqa: E402
from rtfm.llm import MockLLMClient  # noqa: E402


@pytest.fixture
def client():
    cfg = Config(embed_dim=128, chunk_tokens=40, chunk_overlap=8, top_k=3)
    agent = RTFMAgent.in_memory(
        MockLLMClient("Use git commit -m. [1]"), cfg=cfg, embedder=HashEmbedder(dim=cfg.embed_dim)
    )
    app = create_app(cfg=cfg, agent=agent)
    return TestClient(app)


def test_health(client):
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"


def test_ingest_then_ask(client):
    docs = {"git.md": "## COMMIT\nThe git commit command records changes to the repository."}
    r = client.post("/ingest", json={"documents": docs})
    assert r.status_code == 200
    assert r.json()["chunks_added"] >= 1

    r = client.post("/ask", json={"question": "How do I commit changes?"})
    assert r.status_code == 200
    body = r.json()
    assert body["found"] is True
    assert body["answer"]
    assert body["citations"]


def test_ingest_requires_input(client):
    r = client.post("/ingest", json={})
    assert r.status_code == 400


def test_metrics(client):
    client.post("/ingest", json={"documents": {"git.md": "## COMMIT\ngit commit records changes."}})
    client.post("/ask", json={"question": "How do I commit?"})
    r = client.get("/metrics")
    assert r.status_code == 200
    assert "cache" in r.json()
