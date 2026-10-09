from ccctx import graph


def names(hits):
    return {h.qualname for h in hits}


def test_resolve(indexed):
    assert names(graph.resolve(indexed, "dispatch")) == {"Router.dispatch"}
    assert names(graph.resolve(indexed, "Router.dispatch")) == {"Router.dispatch"}
    assert names(graph.resolve(indexed, "app/views.py::index")) == {"index"}


def test_callers(indexed):
    assert names(graph.callers(indexed, "make_response")) == {"not_found", "index", "profile"}
    assert names(graph.callers(indexed, "build_url")) == {"profile", "test_build_url"}
    assert names(graph.callers(indexed, "NewServer")) == {"main"}


def test_calls(indexed):
    resolved, external = graph.calls(indexed, "Router.dispatch")
    assert "not_found" in names(resolved)
    assert "get" in external


def test_impact_hops(indexed):
    hits = {h.qualname: h.hops for h in graph.impact(indexed, "make_response", depth=3)}
    assert hits["not_found"] == 1 and hits["profile"] == 1
    assert hits["Router.dispatch"] == 2       # dispatch -> not_found -> make_response
    shallow = {h.qualname for h in graph.impact(indexed, "make_response", depth=1)}
    assert "Router.dispatch" not in shallow


def test_impact_handles_cycles(indexed, sample):
    from ccctx.ingest import ingest
    (sample / "app/cycle.py").write_text("def a():\n    b()\n\n\ndef b():\n    a()\n")
    ingest(indexed, "sample")
    hits = {h.qualname: h.hops for h in graph.impact(indexed, "a", depth=5)}
    assert hits == {"b": 1, "a": 2}


def test_resolve_import():
    paths = {"app/__init__.py", "app/routing.py", "app/views.py", "web/http.ts", "web/client.ts",
             "internal/json/json.go", "main.go"}
    assert graph.resolve_import(".routing", "app/views.py", "python", paths) == "app/routing.py"
    assert graph.resolve_import(".", "app/views.py", "python", paths) == "app/__init__.py"
    assert graph.resolve_import("app.routing", "x.py", "python", paths) == "app/routing.py"
    assert graph.resolve_import("os", "x.py", "python", paths) is None
    assert graph.resolve_import("./http", "web/client.ts", "typescript", paths) == "web/http.ts"
    assert graph.resolve_import("example.com/m/internal/json", "main.go", "go", paths,
                                go_module="example.com/m") == "internal/json/json.go"


def test_file_dependencies(indexed):
    deps = graph.file_dependencies(indexed, "sample")
    assert deps["app/views.py"] == {"app/routing.py"}
