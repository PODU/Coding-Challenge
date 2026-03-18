"""Natural-language → cron mapping, fallback + LLM path + confirmation."""

from __future__ import annotations

import pytest

from ai_agent_scheduling.cron import CronParseError, CronParser
from ai_agent_scheduling.llm.client import LLMResult, MockLLMClient


@pytest.mark.parametrize(
    "text,expected",
    [
        ("every weekday at 9am", "0 9 * * 1-5"),
        ("every day at 9am", "0 9 * * *"),
        ("hourly", "0 * * * *"),
        ("every hour", "0 * * * *"),
        ("every 5 minutes", "*/5 * * * *"),
        ("every minute", "* * * * *"),
        ("every 2 hours", "0 */2 * * *"),
        ("daily", "0 0 * * *"),
        ("daily at midnight", "0 0 * * *"),
        ("every day at noon", "0 12 * * *"),
        ("weekly", "0 0 * * 0"),
        ("every monday at 9:30am", "30 9 * * 1"),
        ("every weekday at 17:00", "0 17 * * 1-5"),
        ("at 5pm", "0 17 * * *"),
        ("every saturday and sunday at 8am", "0 8 * * 0,6"),
    ],
)
def test_fallback_maps_known_phrases(text, expected):
    parser = CronParser()  # no LLM needed for the deterministic path
    result = parser.parse(text)
    assert result.cron == expected
    assert result.source == "fallback"


def test_confirmation_string_is_shown():
    parser = CronParser()
    result = parser.parse("every weekday at 9am")
    assert "`0 9 * * 1-5`" in result.confirmation
    assert "09:00" in result.confirmation


def test_raw_cron_is_accepted_and_normalised():
    parser = CronParser()
    result = parser.parse("0   9 * * 1-5")
    assert result.cron == "0 9 * * 1-5"


def test_llm_path_consumes_scripted_json_and_validates():
    llm = MockLLMClient(
        replies=[LLMResult(text='{"cron": "15 8 * * 1-5", "explanation": "weekdays 8:15"}', model="mock")]
    )
    parser = CronParser(llm)
    # A phrase the deterministic fallback can't handle -> goes to the LLM.
    result = parser.parse("first thing on working days but a bit past eight")
    assert result.cron == "15 8 * * 1-5"
    assert result.source == "llm"
    assert "weekdays" in result.explanation


def test_llm_invalid_cron_is_rejected():
    llm = MockLLMClient(replies=[LLMResult(text='{"cron": "99 9 * * *"}', model="mock")])
    parser = CronParser(llm)
    with pytest.raises(CronParseError):
        parser.parse("some unparseable phrasing the fallback cannot map")


def test_ambiguous_input_without_llm_is_rejected():
    parser = CronParser()  # no LLM
    with pytest.raises(CronParseError):
        parser.parse("whenever it feels right")


def test_llm_low_confidence_null_cron_is_rejected():
    llm = MockLLMClient(
        replies=[LLMResult(text='{"cron": null, "explanation": "too ambiguous"}', model="mock")]
    )
    parser = CronParser(llm)
    with pytest.raises(CronParseError) as exc:
        parser.parse("do it sometime that makes sense")
    assert "ambiguous" in str(exc.value)
