import json

import pytest

from reviewagent.providers import make_provider
from reviewagent.providers.anthropic import AnthropicProvider
from reviewagent.providers.mock import MockProvider
from reviewagent.providers.ollama import OllamaProvider
from reviewagent.providers.openai import OpenAIProvider


def test_factory_dispatch():
    assert isinstance(make_provider("mock"), MockProvider)
    assert isinstance(make_provider("anthropic", api_key="k"), AnthropicProvider)
    assert isinstance(make_provider("openai", api_key="k"), OpenAIProvider)
    assert isinstance(make_provider("ollama"), OllamaProvider)
    with pytest.raises(ValueError):
        make_provider("skynet")


def test_anthropic_request_and_parse():
    seen = {}

    def transport(url, method="GET", headers=None, payload=None, timeout=0):
        seen.update(url=url, headers=headers, payload=payload)
        return {"content": [{"type": "text", "text": "hi"}],
                "usage": {"input_tokens": 10, "output_tokens": 2},
                "model": "claude-sonnet-5"}

    p = AnthropicProvider(api_key="k", model="claude-sonnet-5",
                          transport=transport)
    c = p.complete("sys", "user")
    assert c.text == "hi" and c.input_tokens == 10
    assert seen["headers"]["x-api-key"] == "k"
    assert seen["payload"]["system"] == "sys"


def test_openai_request_and_parse():
    def transport(url, method="GET", headers=None, payload=None, timeout=0):
        assert payload["messages"][0]["role"] == "system"
        return {"choices": [{"message": {"content": "resp"}}],
                "usage": {"prompt_tokens": 5, "completion_tokens": 1}}

    c = OpenAIProvider(api_key="k", transport=transport).complete("s", "u")
    assert c.text == "resp" and c.input_tokens == 5


def test_ollama_request_and_parse():
    def transport(url, method="GET", headers=None, payload=None, timeout=0):
        assert payload["stream"] is False
        return {"message": {"content": "local"},
                "prompt_eval_count": 3, "eval_count": 2}

    c = OllamaProvider(transport=transport).complete("s", "u")
    assert c.text == "local" and c.output_tokens == 2


def test_mock_provider_output_is_valid_review_json():
    prompt = ("## Changed files\n### app/config.py\n```diff\n"
              '+API_KEY = "sk_live_abcdef12345678"\n```\n')
    c = MockProvider().complete("sys", prompt)
    data = json.loads(c.text)
    assert data["issues"][0]["severity"] == "CRITICAL"
    assert data["issues"][0]["file"] == "app/config.py"
    assert data["verdict"] == "REQUEST_CHANGES"


def test_mock_provider_clean_diff_approves():
    prompt = "## Changed files\n### README.md\n```diff\n+# Title fix\n```\n"
    data = json.loads(MockProvider().complete("s", prompt).text)
    assert data["issues"] == []
    assert data["verdict"] == "APPROVE"
