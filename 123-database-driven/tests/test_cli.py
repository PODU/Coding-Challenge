from __future__ import annotations

import json

import pytest

from llmwiki import HashingEmbedder, ScriptedLLM, SqliteStore, Wiki
from llmwiki.cli import main
from tests.conftest import ADA_DOC


@pytest.fixture
def cli_wiki(tmp_path):
    return Wiki(
        store=SqliteStore(":memory:"),
        embedder=HashingEmbedder(dim=64),
        llm=ScriptedLLM(),
        projects_root=tmp_path,
    )


def run(argv, wiki, capsys):
    code = main(argv, wiki=wiki)
    out = capsys.readouterr().out
    return code, out


def test_cli_project_and_ingest_ask_flow(cli_wiki, tmp_path, capsys):
    code, _ = run(["project", "create", "p"], cli_wiki, capsys)
    assert code == 0

    doc = tmp_path / "ada.txt"
    doc.write_text(ADA_DOC, encoding="utf-8")
    code, out = run(["ingest", "p", str(doc), "--json"], cli_wiki, capsys)
    assert code == 0
    payload = json.loads(out)
    assert payload["pages_created"]
    assert payload["ops"] >= 2

    code, out = run(["ask", "p", "Who was Ada Lovelace?", "--json"], cli_wiki, capsys)
    assert code == 0
    ans = json.loads(out)
    assert "answer" in ans and "citations" in ans and "retrieved" in ans

    code, out = run(["browse", "p", "--type", "entity", "--json"], cli_wiki, capsys)
    assert code == 0
    pages = json.loads(out)
    assert all(p["type"] == "entity" for p in pages)

    code, out = run(["log", "p", "--json"], cli_wiki, capsys)
    assert code == 0
    ops = json.loads(out)
    assert isinstance(ops, list) and ops and ops[0]["seq"] == 1


def test_cli_lint_json(cli_wiki, capsys):
    run(["project", "create", "p"], cli_wiki, capsys)
    cli_wiki.ingest_text("p", ADA_DOC, title="Ada")
    code, out = run(["lint", "p", "--json"], cli_wiki, capsys)
    assert code == 0
    findings = json.loads(out)
    assert isinstance(findings, list)


def test_cli_project_list(cli_wiki, capsys):
    run(["project", "create", "alpha"], cli_wiki, capsys)
    run(["project", "create", "beta"], cli_wiki, capsys)
    code, out = run(["project", "list"], cli_wiki, capsys)
    assert code == 0
    assert "alpha" in out and "beta" in out


def test_cli_delete_requires_force(cli_wiki, capsys):
    run(["project", "create", "p"], cli_wiki, capsys)
    code = main(["project", "delete", "p"], wiki=cli_wiki)
    assert code == 1  # refused without --force
    code = main(["project", "delete", "p", "--force"], wiki=cli_wiki)
    assert code == 0
    assert "p" not in cli_wiki.list_projects()
