import time

from ccctx import memory
from ccctx.embed import HashEmbedder, stem, terms
from ccctx.search import (MAX_BOOST, boost_from_weight, feedback_weights, fts_query, identifier_tokens,
                          is_test_path, search)


def quals(results):
    return [r.qualname for r in results]


def test_embedder_is_normalised_and_deterministic():
    e = HashEmbedder()
    a, b = e.embed(["route the request", "route the request"])
    assert abs(float((a * a).sum()) - 1.0) < 1e-5
    assert (a == b).all()


def test_stemming_and_concepts():
    assert {stem(w) for w in ("route", "routes", "routed", "routing", "router")} == {"rout"}
    t = terms("handleHTTPRequest")
    assert "handl" in t and "request" in t and "@handler" in t


def test_semantic_search_finds_router(indexed):
    res = search(indexed, "how are incoming requests routed to handlers", mode="vector")
    assert res and len(res) <= 10
    assert all(r.similarity >= 0.3 for r in res)
    assert res[0].qualname in ("Router", "Router.dispatch", "Router.add_route", "Server.ServeHTTP")


def test_unrelated_query_returns_nothing(indexed):
    for q in ("kubernetes pod autoscaling", "train a convolutional neural network"):
        assert search(indexed, q, mode="vector") == []
        assert search(indexed, q, mode="hybrid") == []


def test_identifier_tokens():
    assert identifier_tokens("url_for") == ["url_for"]
    assert identifier_tokens("where is Flask.url_for used") == ["Flask.url_for"]
    assert identifier_tokens("build a url for an endpoint") == []
    assert identifier_tokens("who calls handleHTTPRequest()") == ["handleHTTPRequest"]


def test_hybrid_ranks_definition_above_call_sites(indexed):
    res = search(indexed, "build_url", mode="hybrid")
    assert res[0].qualname == "build_url" and res[0].definition
    callers = [r.qualname for r in res if r.qualname in ("profile", "test_build_url")]
    assert callers  # call sites still found, just lower


def test_text_mode_and_fts_query():
    q = fts_query("routed handlers")
    assert '"routed"' in q and '"rout"*' in q and '"handl"*' in q


def test_text_mode_finds_identifier(indexed):
    res = search(indexed, "make_response", mode="text")
    assert "make_response" in quals(res)


def test_repo_and_language_filters(indexed, sample, tmp_path):
    from ccctx.ingest import ingest
    from conftest import write_sample
    other = write_sample(tmp_path / "other")
    indexed.add_repo("other", str(other))
    ingest(indexed, "other")
    res = search(indexed, "server", repo="other")
    assert res and all(r.repo == "other" for r in res)
    res = search(indexed, "server request", language="go")
    assert res and all(r.language == "go" for r in res)


def test_tests_are_demoted_unless_asked(indexed):
    assert is_test_path("tests/test_views.py") and is_test_path("pkg/x_test.go")
    assert is_test_path("web/a.spec.ts")
    assert not is_test_path("app/views.py") and not is_test_path("app/contest.py")
    res = quals(search(indexed, "build url", use_feedback=False))
    assert res.index("build_url") < res.index("test_build_url")
    res = quals(search(indexed, "test build url", use_feedback=False))
    assert res[0] == "test_build_url"


def test_feedback_boost_is_bounded_and_decays(indexed):
    now = time.time()
    key = "app/views.py::build_url"
    for _ in range(50):
        memory.record_feedback(indexed, "sample", key, "open", ts=now)
    w = feedback_weights(indexed, [key], now)[key]
    assert 49 < w <= 50
    assert boost_from_weight(w) <= MAX_BOOST
    # 30-day half-life
    w_later = feedback_weights(indexed, [key], now + 30 * 86400)[key]
    assert abs(w_later - w / 2) < 0.01
    res = search(indexed, "build a url for an endpoint", now=now)
    hit = next(r for r in res if r.key == key)
    assert 0 < hit.boost <= MAX_BOOST


def test_feedback_never_injects_irrelevant_results(indexed):
    key = "app/storage.py::save_user"
    for _ in range(100):
        memory.record_feedback(indexed, "sample", key, "open")
    res = search(indexed, "how are incoming requests routed to handlers")
    assert key not in [r.key for r in res]
    assert search(indexed, "kubernetes pod autoscaling") == []


def test_feedback_reorders_within_relevant_results(indexed):
    base = search(indexed, "user database", use_feedback=False)
    assert len(base) >= 2
    target = base[-1]
    for _ in range(30):
        memory.record_feedback(indexed, "sample", target.key, "open")
    boosted = search(indexed, "user database")
    assert [r.key for r in boosted].index(target.key) <= [r.key for r in base].index(target.key)
