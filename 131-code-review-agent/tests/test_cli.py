import json

import pytest

from reviewagent import cli, http
from conftest import FakeTransport


@pytest.fixture
def wired(monkeypatch):
    """Point every HTTP call at the fake GitHub, force mock provider."""
    transport = FakeTransport()
    monkeypatch.setattr(http, "request_json", transport)
    monkeypatch.setenv("GITHUB_TOKEN", "tok")
    monkeypatch.setenv("REVIEW_PROVIDER", "mock")
    return transport


def test_missing_token_exits_2(monkeypatch, capsys):
    monkeypatch.delenv("GITHUB_TOKEN", raising=False)
    assert cli.main(["o", "r", "1"]) == 2
    assert "GITHUB_TOKEN" in capsys.readouterr().err


def test_dry_run_prints_comment_posts_nothing(wired, capsys):
    assert cli.main(["o", "r", "1", "--dry-run"]) == 0
    out = capsys.readouterr().out
    assert "Automated Code Review" in out
    assert "CRITICAL" in out
    assert wired.comments == []


def test_real_run_posts(wired):
    assert cli.main(["o", "r", "1"]) == 0
    assert len(wired.comments) == 1


def test_json_output(wired, capsys):
    assert cli.main(["o", "r", "1", "--dry-run", "--json"]) == 0
    data = json.loads(capsys.readouterr().out)
    assert data["issues"] >= 1
    assert data["posted"] is False
    assert "uv.lock" in [s[0] for s in data["skipped"]]
