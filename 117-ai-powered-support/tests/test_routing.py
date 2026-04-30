"""Cost-aware routing: classify simple/complex and pick the right model."""

from __future__ import annotations

from support_bot.llm.mock import MockLLMClient
from support_bot.llm.routing import QueryClassifier, RoutingLLMClient
from support_bot.models import Message, Prompt


def _prompt(text: str) -> Prompt:
    return Prompt(system="", messages=[Message("user", text)])


def test_classifier_labels_simple_and_complex():
    clf = QueryClassifier()
    assert clf.classify("What is the wc challenge?") == "simple"
    assert clf.classify("List the available challenges") == "simple"
    assert clf.classify("Why should I build my own Redis and how does it compare to memcached?") == "complex"
    assert clf.classify("Explain the architecture of the load balancer challenge") == "complex"


def test_routing_picks_cheap_model_for_simple_query():
    mock = MockLLMClient()
    router = RoutingLLMClient(mock, simple_model="claude-haiku-4-5", complex_model="claude-opus-4-8")
    router.complete(_prompt("What is the wc challenge?"))
    assert mock.requested_models[-1] == "claude-haiku-4-5"


def test_routing_picks_capable_model_for_complex_query():
    mock = MockLLMClient()
    router = RoutingLLMClient(mock, simple_model="claude-haiku-4-5", complex_model="claude-opus-4-8")
    router.complete(_prompt("Explain how the Redis challenge handles concurrency and why"))
    assert mock.requested_models[-1] == "claude-opus-4-8"


def test_explicit_model_overrides_routing():
    mock = MockLLMClient()
    router = RoutingLLMClient(mock, simple_model="claude-haiku-4-5", complex_model="claude-opus-4-8")
    router.complete(_prompt("anything"), model="claude-sonnet-4-6")
    assert mock.requested_models[-1] == "claude-sonnet-4-6"
