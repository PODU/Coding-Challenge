"""The ``web_search`` tool.

``DuckDuckGoSearcher`` queries the DuckDuckGo Instant Answer API (no key
needed). ``MockWebSearcher`` returns canned results for tests and the demo.
"""

from __future__ import annotations

import json
import urllib.parse
import urllib.request
from typing import Protocol

from ..llm import ToolSpec
from . import ToolError, ToolRegistry


class WebSearcher(Protocol):
    def search(self, query: str, max_results: int = 5) -> list[dict]: ...


class MockWebSearcher:
    def __init__(self, results: dict[str, list[dict]] | None = None) -> None:
        self._results = results or {}
        self.queries: list[str] = []

    def search(self, query: str, max_results: int = 5) -> list[dict]:
        self.queries.append(query)
        for key, hits in self._results.items():
            if key.lower() in query.lower():
                return hits[:max_results]
        return [
            {
                "title": f"Mock result for {query!r}",
                "url": "https://example.com/mock",
                "snippet": "Offline mock search result.",
            }
        ][:max_results]


class DuckDuckGoSearcher:
    ENDPOINT = "https://api.duckduckgo.com/?format=json&no_html=1&q="

    def __init__(self, timeout: float = 10.0) -> None:
        self.timeout = timeout

    def search(self, query: str, max_results: int = 5) -> list[dict]:
        url = self.ENDPOINT + urllib.parse.quote(query)
        try:
            with urllib.request.urlopen(url, timeout=self.timeout) as resp:
                data = json.loads(resp.read().decode("utf-8"))
        except OSError as exc:
            raise ToolError(f"web search failed: {exc}") from exc
        results: list[dict] = []
        if data.get("AbstractText"):
            results.append(
                {
                    "title": data.get("Heading", query),
                    "url": data.get("AbstractURL", ""),
                    "snippet": data["AbstractText"],
                }
            )
        for topic in data.get("RelatedTopics", []):
            if "Text" in topic and "FirstURL" in topic:
                results.append(
                    {
                        "title": topic["Text"][:80],
                        "url": topic["FirstURL"],
                        "snippet": topic["Text"],
                    }
                )
            if len(results) >= max_results:
                break
        return results[:max_results]


def register(registry: ToolRegistry, searcher: WebSearcher) -> None:
    registry.register(
        ToolSpec(
            name="web_search",
            description="Search the web for external context about a topic.",
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "max_results": {"type": "integer", "default": 5},
                },
                "required": ["query"],
            },
        ),
        lambda query, max_results=5: {"results": searcher.search(query, max_results)},
    )
