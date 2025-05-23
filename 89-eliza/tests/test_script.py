"""Tests for script loading & validation."""

from __future__ import annotations

import json

import pytest

from eliza.script import Script, ScriptError

_MINIMAL = {
    "synonyms": {"fam": ["mother", "father"]},
    "keywords": [
        {"keyword": "xnone", "rules": [{"decomp": "*", "reassemb": ["I see."]}]},
    ],
}


def _with_keyword(extra: dict) -> str:
    data = json.loads(json.dumps(_MINIMAL))
    data["keywords"].append(extra)
    return json.dumps(data)


def test_loads_bundled_doctor_with_xnone() -> None:
    script = Script.load_default()
    assert "xnone" in script.keywords
    assert script.keywords["computer"].rank == 50


def test_malformed_json_raises() -> None:
    with pytest.raises(ScriptError):
        Script.loads("{ not valid json ]")


def test_missing_xnone_raises() -> None:
    data = {"keywords": [{"keyword": "hi", "rules": [{"decomp": "*", "reassemb": ["hello"]}]}]}
    with pytest.raises(ScriptError):
        Script.loads(json.dumps(data))


def test_delegate_to_unknown_keyword_raises() -> None:
    text = _with_keyword({"keyword": "how", "rules": [{"decomp": "*", "delegate": "nowhere"}]})
    with pytest.raises(ScriptError):
        Script.loads(text)


def test_placeholder_out_of_range_raises() -> None:
    text = _with_keyword(
        {"keyword": "my", "rules": [{"decomp": "* my *", "reassemb": ["Your (3)?"]}]}
    )
    with pytest.raises(ScriptError):
        Script.loads(text)


def test_unknown_class_in_decomp_raises() -> None:
    text = _with_keyword(
        {"keyword": "my", "rules": [{"decomp": "* @missing *", "reassemb": ["x"]}]}
    )
    with pytest.raises(ScriptError):
        Script.loads(text)


def test_self_delegation_raises() -> None:
    text = _with_keyword({"keyword": "loop", "rules": [{"decomp": "*", "delegate": "loop"}]})
    with pytest.raises(ScriptError):
        Script.loads(text)


def test_duplicate_keyword_raises() -> None:
    data = json.loads(json.dumps(_MINIMAL))
    data["keywords"].append({"keyword": "xnone", "rules": [{"decomp": "*", "reassemb": ["x"]}]})
    with pytest.raises(ScriptError):
        Script.loads(json.dumps(data))
