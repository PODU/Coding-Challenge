import shutil
import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from ccctx.ingest import ingest  # noqa: E402
from ccctx.store import Store  # noqa: E402

HAS_GIT = shutil.which("git") is not None

ROUTING = '''"""Request routing for the sample app."""


class Router:
    """Routes incoming requests to handler functions."""

    def __init__(self):
        self.rules = {}

    def add_route(self, path, handler):
        """Register a handler for a URL path."""
        self.rules[path] = handler

    def dispatch(self, request):
        """Find the handler for a request path and call it."""
        handler = self.rules.get(request.path)
        if handler is None:
            return not_found(request)
        return handler(request)


def not_found(request):
    """Return a 404 response."""
    return make_response(404, "not found")


def make_response(status, body):
    """Wrap a status code and body into a response."""
    return {"status": status, "body": body}
'''

VIEWS = '''from .routing import Router, make_response


def index(request):
    """The home page."""
    return make_response(200, "hello")


def build_url(endpoint, **values):
    """Build a URL for the given endpoint."""
    return "/" + endpoint


def profile(request):
    """Show the user profile page."""
    link = build_url("profile")
    return make_response(200, link)


def setup(router):
    router.add_route("/", index)
    router.add_route("/profile", profile)
'''

STORAGE = '''"""Persistence helpers."""


class Database:
    """A tiny database connection wrapper."""

    def connect(self, dsn):
        """Open a connection to the database."""
        self.dsn = dsn

    def query(self, sql):
        """Run a SQL query and return rows."""
        return []

    def close(self):
        self.dsn = None


def save_user(db, user):
    """Persist a user record to the database."""
    db.query("insert")


def load_user(db, user_id):
    """Load a user record by id."""
    return db.query("select")
'''

TESTS = '''from app.views import build_url, index


def test_index():
    assert index(None)["status"] == 200


def test_build_url():
    assert build_url("x") == "/x"
'''

SERVER_GO = '''package main

import (
\t"fmt"
\t"net/http"
)

// Server serves HTTP requests.
type Server struct {
\tname string
}

// ServeHTTP handles one incoming request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
\tfmt.Fprintln(w, s.greet(r.URL.Path))
}

func (s *Server) greet(path string) string {
\treturn "hello " + path
}

// NewServer creates a server.
func NewServer(name string) *Server {
\treturn &Server{name: name}
}

func main() {
\thttp.ListenAndServe(":8080", NewServer("demo"))
}
'''

CLIENT_TS = '''import { request } from "./http";

/** Talks to the user API. */
export class ApiClient {
  constructor(private base: string) {}

  async fetchUser(id: number): Promise<User> {
    const res = await request(`${this.base}/users/${id}`);
    return parseUser(res);
  }
}

export interface User { id: number; name: string }

export function parseUser(raw: any): User {
  return { id: raw.id, name: formatName(raw.name) };
}

export const formatName = (s: string) => s.trim();
'''


def write_sample(root: Path) -> Path:
    files = {
        "app/__init__.py": '"""Sample application package."""\n',
        "app/routing.py": ROUTING,
        "app/views.py": VIEWS,
        "app/storage.py": STORAGE,
        "tests/test_views.py": TESTS,
        "server.go": SERVER_GO,
        "web/client.ts": CLIENT_TS,
        "README.md": "# Sample\n\n" + "\n".join(f"Line {i} of the readme." for i in range(1, 131)) + "\n",
        ".gitignore": "build/\n*.log\n!keep.log\n",
        "build/out.txt": "generated\n",
        "debug.log": "noise\n",
        "keep.log": "kept log\n",
    }
    for rel, text in files.items():
        p = root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text, encoding="utf-8", newline="\n")
    (root / "logo.png").write_bytes(b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
    return root


def git(root: Path, *args: str) -> str:
    out = subprocess.run(["git", "-C", str(root), "-c", "user.name=t", "-c", "user.email=t@example.com",
                          "-c", "commit.gpgsign=false", *args], capture_output=True, text=True, check=True)
    return out.stdout.strip()


def commit_all(root: Path, msg: str = "c") -> str:
    git(root, "add", "-A")
    git(root, "commit", "-q", "-m", msg)
    return git(root, "rev-parse", "HEAD")


@pytest.fixture
def sample(tmp_path) -> Path:
    root = write_sample(tmp_path / "sample")
    if HAS_GIT:
        git(root, "init", "-q")
        commit_all(root, "initial")
    return root


@pytest.fixture
def store(tmp_path):
    st = Store(tmp_path / "db" / "ccctx.db")
    yield st
    st.close()


@pytest.fixture
def indexed(store, sample):
    store.add_repo("sample", str(sample))
    ingest(store, "sample")
    return store
