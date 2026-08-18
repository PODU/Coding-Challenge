import pytest

from reviewagent.review import (
    Issue,
    ParseError,
    Review,
    format_comment,
    merge_reviews,
    parse_review,
    should_post,
)


def test_parse_plain_json():
    r = parse_review('{"summary": "s", "issues": [], "verdict": "approve"}')
    assert r.summary == "s"
    assert r.verdict == "APPROVE"
    assert r.issues == []


def test_parse_json_in_fences_and_prose():
    text = 'Here is my review:\n```json\n{"summary": "x", "issues": [' \
           '{"severity": "critical", "file": "a.py", "line": 3, ' \
           '"title": "t", "detail": "d"}]}\n```\nThanks!'
    r = parse_review(text)
    assert r.issues[0].severity == "CRITICAL"
    assert r.issues[0].line == 3


def test_parse_unknown_severity_downgraded():
    r = parse_review('{"issues": [{"severity": "BLOCKER", "file": "a", '
                     '"title": "t", "detail": "d"}]}')
    assert r.issues[0].severity == "MINOR"


def test_parse_garbage_raises():
    with pytest.raises(ParseError):
        parse_review("I could not review this file, sorry.")


def test_parse_default_file_applied():
    r = parse_review('{"issues": [{"severity": "MAJOR", "title": "t", '
                     '"detail": "d"}]}', default_file="app/x.py")
    assert r.issues[0].file == "app/x.py"


def _issue(sev, file="f.py"):
    return Issue(severity=sev, file=file, line=None, title=sev.lower(), detail="")


def test_merge_sorts_critical_first():
    merged = merge_reviews([
        Review(issues=[_issue("MINOR")]),
        Review(issues=[_issue("CRITICAL")]),
        Review(issues=[_issue("MAJOR")]),
    ])
    assert [i.severity for i in merged.issues] == ["CRITICAL", "MAJOR", "MINOR"]
    assert merged.verdict == "REQUEST_CHANGES"


def test_merge_all_clean_is_approve():
    merged = merge_reviews([Review(summary="a"), Review(summary="b")])
    assert merged.verdict == "APPROVE"
    assert not merged.issues


def test_merge_only_minor_is_comment():
    merged = merge_reviews([Review(issues=[_issue("MINOR")])])
    assert merged.verdict == "COMMENT"


def test_should_post_only_with_issues():
    assert not should_post(Review())
    assert should_post(Review(issues=[_issue("MINOR")]))


def test_format_comment_contents():
    review = Review(summary="Sum.", issues=[_issue("CRITICAL", "app/config.py")])
    body = format_comment(review, skipped=[("uv.lock", "lock file")],
                          reviewed_files=["app/config.py"])
    assert "Automated Code Review" in body
    assert "CRITICAL" in body and "app/config.py" in body
    assert "uv.lock" in body and "lock file" in body
    assert "REQUEST_CHANGES" not in body or review.verdict in body
