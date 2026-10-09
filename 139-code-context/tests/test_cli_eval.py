import json

from conftest import HAS_GIT

from ccctx import evaluate, oracle
from ccctx.cli import main

GOLD = {
    "repo": "sample", "k": 5,
    "questions": [
        {"q": "route a request to its handler", "expected": ["Router.dispatch"]},
        {"q": "build a url for an endpoint", "expected": ["build_url"]},
        {"q": "save a user record", "expected": ["save_user"]},
        {"q": "open a database connection", "expected": ["Database.connect"]},
        {"q": "kubernetes pod autoscaling", "expected": []},
    ],
}


def run(capsys, db, *argv):
    code = main(["--db", str(db), *argv])
    out = capsys.readouterr()
    return code, out.out, out.err


def test_cli_end_to_end(tmp_path, sample, capsys):
    db = tmp_path / "cli.db"
    assert run(capsys, db, "repo", "add", str(sample), "--name", "sample")[0] == 0
    code, out, _ = run(capsys, db, "repo", "list")
    assert "sample" in out and " 0 " in out
    code, out, _ = run(capsys, db, "ingest", "sample")
    assert code == 0 and "Scanned" in out and "Indexed" in out
    if HAS_GIT:
        assert "nothing to do" in run(capsys, db, "ingest", "sample")[1]
    assert "full" not in run(capsys, db, "ingest", "sample", "--full")[2]
    code, out, _ = run(capsys, db, "symbols", "--repo", "sample", "--limit", "5")
    assert code == 0 and len(out.strip().splitlines()) == 5
    assert "python" in run(capsys, db, "langs")[1]
    code, out, _ = run(capsys, db, "search", "build_url", "--mode", "hybrid")
    assert "1." in out and "build_url" in out.splitlines()[0]
    assert "== hybrid" in run(capsys, db, "search", "router", "--compare")[1]
    assert "No results" in run(capsys, db, "search", "kubernetes pod autoscaling")[1]
    assert "def build_url" in run(capsys, db, "get", "build_url")[1]
    assert "profile" in run(capsys, db, "callers", "build_url")[1]
    assert "not_found" in run(capsys, db, "calls", "Router.dispatch")[1]
    out = run(capsys, db, "impact", "make_response", "--depth", "3")[1]
    assert "at hop 2" in out and "[2] Router.dispatch" in out
    assert "+ app" in run(capsys, db, "wiki", "build", "--repo", "sample")[1]
    assert "## Key symbols" in run(capsys, db, "wiki", "show", "app")[1]
    assert "fresh" in run(capsys, db, "wiki", "citations", "app")[1]
    assert "app" in run(capsys, db, "wiki", "list")[1]
    assert "search_code" in run(capsys, db, "memory", "list", "--repo", "sample")[1]
    assert "fact(s) written" in run(capsys, db, "memory", "consolidate", "--repo", "sample")[1]
    assert run(capsys, db, "memory", "facts", "--repo", "sample")[0] == 0
    assert "recorded cite" in run(capsys, db, "feedback", "build_url")[1]
    stats = json.loads(run(capsys, db, "stats")[1])
    assert stats["symbols"] > 0 and stats["runs"] > 0 and stats["feedback"] > 0
    assert run(capsys, db, "ingest", "nope")[0] == 2
    assert run(capsys, db, "get", "does_not_exist")[0] == 1


def test_eval_metrics_and_feedback_isolation(indexed, tmp_path):
    gold = tmp_path / "gold.json"
    gold.write_text(json.dumps(GOLD))
    before = indexed.q1("SELECT COUNT(*) FROM feedback")[0]
    report = evaluate.evaluate(indexed, gold, with_feedback=True)
    assert set(report["modes"]) == {"vector", "text", "hybrid", "hybrid+feedback"}
    hy = report["modes"]["hybrid"].summary()
    fb = report["modes"]["hybrid+feedback"].summary()
    assert hy["recall@k"] >= 0.75
    assert fb["recall@k"] >= hy["recall@k"] and fb["mrr"] >= hy["mrr"]
    assert fb["negative_boosted"] == 0 and hy["negative_results"] == 0
    assert report["feedback_events"] > 0
    assert indexed.q1("SELECT COUNT(*) FROM feedback")[0] == before  # real DB untouched
    text = evaluate.render(report)
    assert "hybrid+feedback" in text and "recall@k" in text


def test_eval_cli(tmp_path, indexed, capsys):
    gold = tmp_path / "gold.json"
    gold.write_text(json.dumps(GOLD))
    code = main(["--db", str(indexed.path), "eval", "--gold", str(gold), "--json"])
    data = json.loads(capsys.readouterr().out)
    assert code == 0 and data["modes"]["hybrid"]["questions"] == 5


def test_oracle_sql_covers_the_spec():
    sql = oracle.script("all")
    for needle in ("DBMS_VECTOR.LOAD_ONNX_MODEL", "VECTOR(384, FLOAT32)", "VECTOR_EMBEDDING(ALL_MINILM_L12_V2",
                   "ORGANIZATION INMEMORY NEIGHBOR GRAPH", "DISTANCE COSINE", "WITH TARGET ACCURACY 95",
                   "DBMS_HYBRID_VECTOR.SEARCH", '"search_scorer"    : "rrf"', '"search_fusion"    : "UNION"',
                   "CREATE OR REPLACE PROPERTY GRAPH", "GRAPH_TABLE", "->{1,3}", ">= 0.3",
                   "ON DELETE SET NULL"):
        assert needle in sql, needle
    assert oracle.script("graph").startswith("-- SQL property graph")
