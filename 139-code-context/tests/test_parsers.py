from conftest import CLIENT_TS, ROUTING, SERVER_GO

from ccctx.langs import detect
from ccctx.parsers import mask_code, parse, window_chunks


def by_qual(parsed):
    return {s.qualname: s for s in parsed.symbols}


def test_language_detection():
    assert detect("a/b.py") == "python"
    assert detect("x.go") == "go"
    assert detect("x.tsx") == "typescript"
    assert detect("x.mjs") == "javascript"
    assert detect("Foo.java") == "java"
    assert detect("lib.rs") == "rust"
    assert detect("README.md") == "markdown"
    assert detect("Makefile") == "make"
    assert detect("script", b"#!/usr/bin/env python3\n") == "python"
    assert detect("unknown.zzz") == "text"


def test_python_functions_methods_classes():
    p = parse("app/routing.py", ROUTING, "python")
    assert p.parser == "ast"
    syms = by_qual(p)
    assert syms["Router"].kind == "class"
    assert syms["Router.dispatch"].kind == "method"
    assert syms["not_found"].kind == "function"
    d = syms["Router.dispatch"]
    assert d.doc.startswith("Find the handler")
    assert d.signature == "def dispatch(self, request)"
    assert "not_found" in d.calls and "get" in d.calls
    assert d.source.lstrip().startswith("def dispatch")
    assert ROUTING.splitlines()[d.start_line - 1].strip().startswith("def dispatch")
    assert p.module_doc == "Request routing for the sample app."


def test_python_decorators_included_in_range():
    src = "import x\n\n@app.route('/')\n@login\ndef home():\n    return 1\n"
    p = parse("a.py", src, "python")
    s = p.symbols[0]
    assert (s.start_line, s.end_line) == (3, 6)
    assert "route" in s.calls and "login" in s.calls
    assert p.imports == ["x"]


def test_python_relative_imports_recorded():
    p = parse("pkg/a.py", "from . import b\nfrom ..c import d\nimport os.path\n\ndef f():\n    pass\n", "python")
    assert p.imports == [".", "..c", "os.path"]


def test_go_structural_parse():
    p = parse("server.go", SERVER_GO, "go")
    assert p.parser == "structural"
    syms = by_qual(p)
    assert syms["Server"].kind == "struct"
    assert syms["Server.ServeHTTP"].kind == "method"
    assert syms["Server.ServeHTTP"].doc == "ServeHTTP handles one incoming request."
    assert "greet" in syms["Server.ServeHTTP"].calls
    assert syms["NewServer"].kind == "function"
    assert "NewServer" in syms["main"].calls
    assert set(p.imports) == {"fmt", "net/http"}


def test_go_interface_braces_in_signature_and_strings():
    src = ('package x\n\nfunc F(v interface{}) interface{} {\n\ts := "}"\n\treturn g(v)\n}\n\n'
           'func G() {\n}\n')
    syms = by_qual(parse("x.go", src, "go"))
    assert (syms["F"].start_line, syms["F"].end_line) == (3, 6)
    assert syms["F"].calls == ["g"]
    assert (syms["G"].start_line, syms["G"].end_line) == (8, 9)


def test_typescript_classes_methods_functions():
    p = parse("web/client.ts", CLIENT_TS, "typescript")
    syms = by_qual(p)
    assert syms["ApiClient"].kind == "class"
    assert syms["ApiClient"].doc == "Talks to the user API."
    assert syms["ApiClient.fetchUser"].kind == "method"
    assert {"request", "parseUser"} <= set(syms["ApiClient.fetchUser"].calls)
    assert syms["User"].kind == "interface"
    assert syms["parseUser"].kind == "function"
    assert syms["formatName"].kind == "function"
    assert p.imports == ["./http"]


def test_javascript_object_methods_are_not_top_level_symbols():
    src = "const api = {\n  get(x) {\n    return 1\n  }\n};\nfunction real() { return api.get(1) }\n"
    syms = by_qual(parse("a.js", src, "javascript"))
    assert "get" not in syms and "real" in syms


def test_java():
    src = """package a;
import java.util.List;

/** A greeter. */
public class Greeter {
    private int count = compute();

    public Greeter(int c) { this.count = c; }

    @Override
    public String greet(String name) throws Exception {
        if (name == null) { return fallback(); }
        return "hi " + name;
    }
}
"""
    p = parse("Greeter.java", src, "java")
    syms = by_qual(p)
    assert syms["Greeter"].kind == "class" and syms["Greeter"].doc == "A greeter."
    assert syms["Greeter.greet"].kind == "method"
    assert syms["Greeter.greet"].calls == ["fallback"]
    assert (syms["Greeter.greet"].start_line, syms["Greeter.greet"].end_line) == (11, 14)
    assert "Greeter.Greeter" in syms
    assert p.imports == ["java.util.List"]


def test_rust_impl_methods_and_lifetimes():
    src = """use std::fmt;

pub struct Point<'a> { name: &'a str }

impl<'a> fmt::Display for Point<'a> {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.name);
        helper('}')
    }
}

trait Shape { fn area(&self) -> f64; }

fn helper(c: char) -> fmt::Result { Ok(()) }
"""
    syms = by_qual(parse("lib.rs", src, "rust"))
    assert syms["Point"].kind == "struct"
    assert syms["Point.fmt"].kind == "method"
    assert (syms["Point.fmt"].start_line, syms["Point.fmt"].end_line) == (6, 9)
    assert syms["Point.fmt"].calls == ["helper"]
    assert syms["Shape"].kind == "trait"
    assert syms["helper"].kind == "function"


def test_mask_code_blanks_strings_and_comments_preserving_length():
    src = 'a = "x{y}" // c {\nb = `t\n{`; /* { */ c\n'
    m = mask_code(src, "javascript")
    assert len(m) == len(src)
    assert m.count("\n") == src.count("\n")
    assert "{" not in m


def test_fallback_windows_60_lines_10_overlap():
    text = "\n".join(f"line {i}" for i in range(1, 131))
    chunks = window_chunks("notes.txt", text)
    assert [(c.start_line, c.end_line) for c in chunks] == [(1, 60), (51, 110), (101, 130)]
    assert all(c.kind == "chunk" for c in chunks)


def test_unparseable_file_falls_back():
    p = parse("broken.py", "def (:\n" * 5, "python")
    assert p.parser == "fallback"
    assert p.symbols and p.symbols[0].kind == "chunk"


def test_markdown_chunks_take_first_heading_as_doc():
    chunks = window_chunks("README.md", "# Title\n\nbody\n", "markdown")
    assert chunks[0].doc == "Title"
