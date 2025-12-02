"""Config loading and CLI wiring — all offline, no key required."""

from __future__ import annotations

import io

import pytest

import chatbot.cli as cli
from chatbot.config import DEFAULT_MODEL, Config
from chatbot.llm import LLMAuthError, LLMResult, Message
from chatbot.persona import BUILT_IN


def test_from_env_reads_key(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-test")
    cfg = Config.from_env(max_tokens=99)
    assert cfg.api_key == "sk-test"
    assert cfg.model == DEFAULT_MODEL
    assert cfg.max_tokens == 99


def test_from_env_raises_without_key(monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    with pytest.raises(LLMAuthError):
        Config.from_env()


def test_parser_defaults():
    args = cli._build_parser().parse_args([])
    assert args.persona == "assistant"
    assert args.stream is True
    assert args.model == DEFAULT_MODEL


def test_no_stream_flag():
    args = cli._build_parser().parse_args(["--no-stream"])
    assert args.stream is False


def test_resolve_persona_builtin():
    args = cli._build_parser().parse_args(["--persona", "pirate"])
    assert cli._resolve_persona(args) == BUILT_IN["pirate"]


def test_resolve_persona_custom_system_overrides():
    args = cli._build_parser().parse_args(["--system", "be terse"])
    persona = cli._resolve_persona(args)
    assert persona.system_prompt == "be terse"
    assert persona.name == "custom"


def test_resolve_persona_system_file(tmp_path):
    p = tmp_path / "prompt.txt"
    p.write_text("from a file", encoding="utf-8")
    args = cli._build_parser().parse_args(["--system-file", str(p)])
    assert cli._resolve_persona(args).system_prompt == "from a file"


def test_main_without_key_returns_one(monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    # avoid importing the real SDK path failing for other reasons
    out = io.StringIO()
    monkeypatch.setattr(cli.sys, "stdout", out)
    code = cli.main([])
    assert code == 1
    assert "ANTHROPIC_API_KEY" in out.getvalue()


class _FakeClient:
    def complete(self, *, system, messages, max_tokens):
        return LLMResult(text="connection ok", stop_reason="end_turn")

    def stream(self, *, system, messages, max_tokens):
        yield "connection ok"


def test_main_selftest(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-test")
    # replace the real SDK-backed client with a fake (main imports it lazily)
    import chatbot.anthropic_client as ac

    monkeypatch.setattr(ac, "AnthropicLLMClient", lambda cfg: _FakeClient())
    out = io.StringIO()
    monkeypatch.setattr(cli.sys, "stdout", out)
    code = cli.main(["--selftest"])
    assert code == 0
    assert "connection ok" in out.getvalue()


def test_main_runs_repl_with_fake_client(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-test")
    import chatbot.anthropic_client as ac

    monkeypatch.setattr(ac, "AnthropicLLMClient", lambda cfg: _FakeClient())
    monkeypatch.setattr(cli.sys, "stdin", io.StringIO("hello\n/exit\n"))
    out = io.StringIO()
    monkeypatch.setattr(cli.sys, "stdout", out)
    code = cli.main(["--persona", "tutor"])
    assert code == 0
    text = out.getvalue()
    assert "connection ok" in text
    assert "bye" in text
