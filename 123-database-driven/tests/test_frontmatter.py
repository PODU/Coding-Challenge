from __future__ import annotations

from llmwiki import Frontmatter, WikiPage
from llmwiki.frontmatter import from_markdown, to_markdown
from llmwiki.text import sha256_hex, slug


def test_frontmatter_round_trip():
    fm = Frontmatter(
        id="entities/ada-lovelace", type="entity", title="Ada Lovelace: Pioneer",
        project="p", sources=["src_a"], links=["summaries/ada"], tags=["computing"],
    )
    page = WikiPage(frontmatter=fm, body="# Ada\n\n- did things")
    page.recompute_sha()
    text = to_markdown(page)
    parsed = from_markdown(text)
    assert parsed.frontmatter.id == fm.id
    assert parsed.frontmatter.title == "Ada Lovelace: Pioneer"  # quoted (has colon)
    assert parsed.frontmatter.sources == ["src_a"]
    assert parsed.frontmatter.links == ["summaries/ada"]
    assert parsed.frontmatter.tags == ["computing"]
    assert parsed.body == page.body


def test_empty_lists_round_trip():
    fm = Frontmatter(id="x", type="topic", title="X", project="p")
    page = WikiPage(frontmatter=fm, body="body")
    parsed = from_markdown(to_markdown(page))
    assert parsed.frontmatter.sources == []
    assert parsed.frontmatter.links == []


def test_sha_stability():
    assert sha256_hex("hello") == sha256_hex("hello")
    assert sha256_hex("hello") != sha256_hex("world")


def test_slug():
    assert slug("Ada Lovelace!") == "ada-lovelace"
    assert slug("  C++  ") == "c"
    assert slug("") == "untitled"
