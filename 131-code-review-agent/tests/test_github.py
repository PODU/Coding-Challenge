import pytest

from reviewagent.github import GitHubClient
from reviewagent.retry import RetryableError


def test_pull_request_metadata(gh):
    pr = gh.pull_request("o", "r", 1)
    assert pr.title == "Add refund support and reporting"
    assert pr.head_sha == "abc123"


def test_changed_files_parsed(gh):
    files = gh.changed_files("o", "r", 1)
    assert len(files) == 4
    names = [f.filename for f in files]
    assert "app/config.py" in names and "uv.lock" in names
    binary = next(f for f in files if f.filename == "assets/logo.png")
    assert binary.patch == ""  # missing patch normalised to empty string


def test_auth_headers_sent(gh, transport):
    gh.pull_request("o", "r", 1)
    headers = transport.requests[0]["headers"]
    assert headers["Authorization"] == "Bearer test-token"
    assert headers["Accept"] == "application/vnd.github+json"


def test_post_comment(gh, transport):
    gh.post_comment("o", "r", 1, "hello **world**")
    assert transport.comments == ["hello **world**"]
    req = transport.requests[-1]
    assert req["method"] == "POST"
    assert "/issues/1/comments" in req["url"]


def test_retries_on_retryable_error():
    calls = {"n": 0}

    def transport(url, **kwargs):
        calls["n"] += 1
        if calls["n"] < 3:
            raise RetryableError("HTTP 502")
        return {"title": "t", "body": "", "head": {"sha": "s"}}

    gh = GitHubClient("tok", transport=transport, sleep=lambda _: None)
    pr = gh.pull_request("o", "r", 1)
    assert pr.title == "t"
    assert calls["n"] == 3


def test_gives_up_after_max_retries():
    def transport(url, **kwargs):
        raise RetryableError("HTTP 503")

    gh = GitHubClient("tok", transport=transport, max_retries=3,
                      sleep=lambda _: None)
    with pytest.raises(RetryableError):
        gh.pull_request("o", "r", 1)


def test_pagination():
    pages = {
        1: [{"filename": f"f{i}.py", "patch": "+x"} for i in range(100)],
        2: [{"filename": "last.py", "patch": "+x"}],
    }

    def transport(url, **kwargs):
        import re
        page = int(re.search(r"[&?]page=(\d+)", url).group(1))
        return pages.get(page, [])

    gh = GitHubClient("tok", transport=transport, sleep=lambda _: None)
    files = gh.changed_files("o", "r", 1)
    assert len(files) == 101
    assert files[-1].filename == "last.py"
