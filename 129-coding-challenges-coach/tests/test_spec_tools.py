"""Reading challenge specs in markdown, JSON and YAML."""

import json

import pytest

from coach.tools.specs import SpecStore, _mini_yaml


def test_list_specs(spec_store):
    names = spec_store.list()
    assert names == ["01-wc.md", "02-json-parser.json", "03-huffman.yaml"]


def test_read_markdown(spec_store):
    result = spec_store.read("01-wc.md")
    assert result["format"] == "md"
    assert result["spec"]["title"] == "Coding Challenge #1: Build Your Own wc"
    assert "-c" in result["spec"]["content"]


def test_read_json(spec_store):
    result = spec_store.read("02-json-parser.json")
    assert result["format"] == "json"
    assert result["spec"]["difficulty"] == "intermediate"
    assert len(result["spec"]["steps"]) == 5


def test_read_yaml(spec_store):
    result = spec_store.read("03-huffman.yaml")
    assert result["format"] == "yaml"
    assert result["spec"]["difficulty"] == "advanced"
    assert "compression" in result["spec"]["topics"]
    assert "Huffman" in result["spec"]["summary"]


def test_missing_spec_is_tool_error_via_registry(registry):
    out = json.loads(registry.call("read_challenge_spec", {"name": "nope.md"}))
    assert "no such spec" in out["error"]


def test_path_escape_blocked(registry):
    out = json.loads(
        registry.call("read_challenge_spec", {"name": "../pyproject.toml"})
    )
    assert "error" in out


def test_mini_yaml_parses_lists_blocks_and_scalars():
    data = _mini_yaml(
        "title: 'T'\nlevel: easy\nitems:\n  - one\n  - two\nbody: |\n  line1\n  line2\n"
    )
    assert data == {
        "title": "T",
        "level": "easy",
        "items": ["one", "two"],
        "body": "line1\nline2\n",
    }


def test_unknown_tool(registry):
    out = json.loads(registry.call("bogus", {}))
    assert "unknown tool" in out["error"]


def test_web_search_tool(registry):
    out = json.loads(registry.call("web_search", {"query": "huffman coding"}))
    assert out["results"][0]["url"].startswith("https://en.wikipedia.org")
