"""CLI smoke tests (offline): create -> list -> status -> delete."""

from __future__ import annotations

import os

import pytest

from ai_agent_scheduling.cli import main


@pytest.fixture
def cli_env(tmp_path, monkeypatch):
    monkeypatch.setenv("DB_PATH", str(tmp_path / "cli.db"))
    monkeypatch.setenv("AI_AGENTS_OFFLINE", "1")  # force the mock LLM
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    monkeypatch.delenv("RESEND_API_KEY", raising=False)
    monkeypatch.delenv("SMTP_HOST", raising=False)
    yield


def test_preview_command(cli_env, capsys):
    rc = main(["preview", "every weekday at 9am"])
    out = capsys.readouterr().out
    assert rc == 0
    assert "0 9 * * 1-5" in out


def test_create_list_run_status_delete(cli_env, capsys):
    rc = main(["create", "demo", "--task", "Say hi", "--schedule", "every weekday at 9am", "-y"])
    assert rc == 0
    assert "Created agent 'demo'" in capsys.readouterr().out

    rc = main(["list"])
    assert rc == 0
    assert "demo" in capsys.readouterr().out

    rc = main(["run", "demo"])
    assert rc == 0
    assert "SUCCESS" in capsys.readouterr().out

    rc = main(["status"])
    out = capsys.readouterr().out
    assert rc == 0
    assert "HEALTHY" in out
    assert "demo" in out

    rc = main(["delete", "demo"])
    assert rc == 0
    assert "Deleted agent 'demo'" in capsys.readouterr().out


def test_create_rejects_unparseable_schedule(cli_env, capsys):
    rc = main(["create", "x", "--task", "t", "--schedule", "whenever", "-y"])
    assert rc == 1


def test_create_duplicate_fails(cli_env, capsys):
    main(["create", "dup", "--task", "t", "--schedule", "hourly", "-y"])
    capsys.readouterr()
    rc = main(["create", "dup", "--task", "t", "--schedule", "hourly", "-y"])
    assert rc == 1
