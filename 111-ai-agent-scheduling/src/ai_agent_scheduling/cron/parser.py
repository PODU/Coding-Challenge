"""Turn English schedule descriptions into cron expressions.

Common phrasings ("every weekday at 9am", "hourly", "every 5 minutes") are
handled with regexes. Anything else goes to the LLM, which must answer in JSON;
its cron is validated before being returned.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import Optional

from ..llm.client import LLMClient
from .expression import CronError, CronExpression, describe

_DOW_WORDS = {
    "sunday": 0, "monday": 1, "tuesday": 2, "wednesday": 3,
    "thursday": 4, "friday": 5, "saturday": 6,
}

_CRON_SYSTEM_PROMPT = (
    "You convert an English description of a schedule into a standard 5-field "
    "cron expression (minute hour day-of-month month day-of-week, day-of-week "
    "0=Sunday). Respond with ONLY a JSON object and nothing else, of the form "
    '{"cron": "<five fields>", "explanation": "<one short sentence>"}. '
    "If the description is too ambiguous to schedule, respond with "
    '{"cron": null, "explanation": "<why>"}.'
)


class CronParseError(ValueError):
    """Raised when a natural-language schedule cannot be turned into valid cron."""


@dataclass(frozen=True)
class CronParseResult:

    cron: str
    explanation: str
    source: str  # "fallback" or "llm"

    @property
    def confirmation(self) -> str:
        return f"Interpreted as `{self.cron}` - {self.explanation}"


class CronParser:
    def __init__(self, llm: Optional[LLMClient] = None, *, timeout_s: float = 30.0):
        self._llm = llm
        self._timeout_s = timeout_s

    def parse(self, text: str, *, allow_llm: bool = True) -> CronParseResult:
        """Return a validated ``CronParseResult`` or raise ``CronParseError``."""
        if not text or not text.strip():
            raise CronParseError("empty schedule description")

        cron = self._fallback(text)
        if cron is not None:
            return CronParseResult(cron=cron, explanation=describe(cron), source="fallback")

        if self._llm is not None and allow_llm:
            return self._via_llm(text)

        raise CronParseError(
            f"could not interpret schedule {text!r}; please rephrase "
            "(e.g. 'every weekday at 9am')"
        )

    def _fallback(self, text: str) -> Optional[str]:
        t = text.strip().lower()

        if CronExpression.is_valid(text):
            return CronExpression.parse(text).raw

        # "every N minutes"
        m = re.search(r"every\s+(\d+)\s*min(?:ute)?s?", t)
        if m:
            n = int(m.group(1))
            if not 1 <= n <= 59:
                return None
            return f"*/{n} * * * *"
        if re.search(r"\bevery\s+minute\b", t):
            return "* * * * *"

        # "every N hours"
        m = re.search(r"every\s+(\d+)\s*hours?", t)
        if m:
            n = int(m.group(1))
            if not 1 <= n <= 23:
                return None
            return f"0 */{n} * * *"
        if re.search(r"\b(hourly|every\s+hour)\b", t):
            return "0 * * * *"

        minute, hour = _parse_time_of_day(t)
        dow = _parse_day_of_week(t)

        if re.search(r"\b(daily|every\s+day|each\s+day)\b", t) and dow is None:
            if hour is None:
                hour, minute = 0, 0
            return f"{minute} {hour} * * *"

        if re.search(r"\b(weekly|every\s+week)\b", t) and dow is None:
            if hour is None:
                hour, minute = 0, 0
            return f"{minute} {hour} * * 0"

        if dow is not None:
            if hour is None:
                hour, minute = 0, 0
            return f"{minute} {hour} * * {dow}"

        if hour is not None:
            # A time with no day means every day.
            return f"{minute} {hour} * * *"

        return None

    def _via_llm(self, text: str) -> CronParseResult:
        assert self._llm is not None
        result = self._llm.complete(
            system=_CRON_SYSTEM_PROMPT,
            task=f"Schedule description: {text}",
            timeout_s=self._timeout_s,
        )
        try:
            payload = json.loads(_extract_json(result.text))
        except (json.JSONDecodeError, ValueError) as exc:
            raise CronParseError(f"LLM returned unparseable response: {exc}") from exc

        cron = payload.get("cron")
        explanation = (payload.get("explanation") or "").strip()
        if not cron:
            raise CronParseError(
                explanation or f"could not interpret schedule {text!r}; please rephrase"
            )
        try:
            cron = CronExpression.parse(cron).raw
        except CronError as exc:
            raise CronParseError(f"LLM produced an invalid cron expression: {exc}") from exc

        return CronParseResult(
            cron=cron,
            explanation=explanation or describe(cron),
            source="llm",
        )


def _parse_time_of_day(t: str) -> tuple[int, Optional[int]]:
    """Return (minute, hour). ``hour`` is None when no time was found."""
    if "midnight" in t:
        return 0, 0
    if "noon" in t or "midday" in t:
        return 0, 12

    m = re.search(r"\b(\d{1,2})(?::(\d{2}))?\s*(am|pm)\b", t)
    if m:
        hour = int(m.group(1)) % 12
        minute = int(m.group(2)) if m.group(2) else 0
        if m.group(3) == "pm":
            hour += 12
        return minute, hour

    # 24-hour clock: "at 17:00", "at 9", "9:30"
    m = re.search(r"\bat\s+(\d{1,2})(?::(\d{2}))?\b", t)
    if not m:
        m = re.search(r"\b(\d{1,2}):(\d{2})\b", t)
        if m:
            return int(m.group(2)), int(m.group(1))
        return 0, None
    hour = int(m.group(1))
    minute = int(m.group(2)) if m.group(2) else 0
    if 0 <= hour <= 23:
        return minute, hour
    return 0, None


def _parse_day_of_week(t: str) -> Optional[str]:
    if "weekday" in t or "week day" in t or "business day" in t:
        return "1-5"
    if "weekend" in t:
        return "0,6"
    found = [str(num) for name, num in _DOW_WORDS.items() if re.search(rf"\b{name}s?\b", t)]
    if found:
        return ",".join(sorted(found, key=int))
    return None


def _extract_json(text: str) -> str:
    """Pull the first ``{...}`` object out of a (possibly chatty) LLM reply."""
    start = text.find("{")
    end = text.rfind("}")
    if start == -1 or end == -1 or end < start:
        raise ValueError("no JSON object found")
    return text[start : end + 1]
