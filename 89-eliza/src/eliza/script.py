"""Loading and validating the JSON script.

Everything is checked at load time, so the engine never meets a bad rule.

In a decomposition pattern, ``*`` matches any number of words, ``@class``
matches one word from a synonym class, and any other word matches itself.
``*`` and ``@class`` are captured, and a template's ``(n)`` refers to the
n-th capture.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from importlib import resources
from typing import Any

__all__ = ["Script", "Keyword", "Rule", "ScriptError"]

# Keyword used when nothing else matches. Every script must have it.
XNONE = "xnone"

_PLACEHOLDER = re.compile(r"\((\d+)\)")


class ScriptError(ValueError):
    """The script is malformed or inconsistent."""


@dataclass
class Rule:
    """A decomposition pattern and its reassembly templates."""

    decomp: str
    regex: re.Pattern[str]
    group_count: int
    reassemb: list[str] = field(default_factory=list)
    delegate: str | None = None


@dataclass
class Keyword:
    keyword: str
    rank: int
    order: int  # position in the file
    rules: list[Rule] = field(default_factory=list)
    memory: list[str] = field(default_factory=list)


@dataclass
class Script:
    """A fully parsed, validated DOCTOR script."""

    initial: list[str]
    final: list[str]
    quit: set[str]
    pre: dict[str, str]
    synonyms: dict[str, list[str]]
    reflections: dict[str, str]
    keywords: dict[str, Keyword]
    memory_size: int = 5

    @classmethod
    def load(cls, path: str) -> "Script":
        try:
            with open(path, "r", encoding="utf-8") as fh:
                text = fh.read()
        except OSError as exc:  # pragma: no cover - trivial I/O wrapper
            raise ScriptError(f"cannot read script file {path!r}: {exc}") from exc
        return cls.loads(text)

    @classmethod
    def load_default(cls) -> "Script":
        """Load the DOCTOR script bundled with the package."""
        text = (
            resources.files("eliza.data")
            .joinpath("doctor.json")
            .read_text(encoding="utf-8")
        )
        return cls.loads(text)

    @classmethod
    def loads(cls, text: str) -> "Script":
        try:
            raw: Any = json.loads(text)
        except json.JSONDecodeError as exc:
            raise ScriptError(f"invalid JSON: {exc}") from exc
        if not isinstance(raw, dict):
            raise ScriptError("script root must be a JSON object")
        return _build(raw)


def _build(raw: dict[str, Any]) -> Script:
    synonyms_raw = raw.get("synonyms", {}) or {}
    if not isinstance(synonyms_raw, dict):
        raise ScriptError("'synonyms' must be an object")
    synonyms: dict[str, list[str]] = {}
    for name, members in synonyms_raw.items():
        if not isinstance(members, list) or not members:
            raise ScriptError(f"synonym class '@{name}' must be a non-empty list")
        synonyms[name] = [str(m).lower() for m in members]

    reflections = {
        str(k).lower(): str(v).lower()
        for k, v in (raw.get("reflections", {}) or {}).items()
    }
    pre = {
        str(k).lower(): str(v).lower() for k, v in (raw.get("pre", {}) or {}).items()
    }

    keywords_raw = raw.get("keywords")
    if not isinstance(keywords_raw, list) or not keywords_raw:
        raise ScriptError("'keywords' must be a non-empty list")

    keywords: dict[str, Keyword] = {}
    for order, entry in enumerate(keywords_raw):
        kw = _build_keyword(entry, order, synonyms)
        if kw.keyword in keywords:
            raise ScriptError(f"duplicate keyword '{kw.keyword}'")
        keywords[kw.keyword] = kw

    if XNONE not in keywords:
        raise ScriptError(f"script must define the '{XNONE}' fallback keyword")

    # Delegation is one hop only, which rules out loops.
    for kw in keywords.values():
        for rule in kw.rules:
            if rule.delegate is None:
                continue
            if rule.delegate not in keywords:
                raise ScriptError(
                    f"keyword '{kw.keyword}' delegates to unknown keyword "
                    f"'{rule.delegate}'"
                )
            if rule.delegate == kw.keyword:
                raise ScriptError(f"keyword '{kw.keyword}' delegates to itself")
            target = keywords[rule.delegate]
            if any(r.delegate is not None for r in target.rules):
                raise ScriptError(
                    f"delegation chain too deep: '{kw.keyword}' -> "
                    f"'{rule.delegate}' which also delegates"
                )

    return Script(
        initial=[str(s) for s in raw.get("initial", ["Hello."])],
        final=[str(s) for s in raw.get("final", ["Goodbye."])],
        quit={str(s).lower() for s in raw.get("quit", ["bye", "quit", "exit"])},
        pre=pre,
        synonyms=synonyms,
        reflections=reflections,
        keywords=keywords,
        memory_size=int(raw.get("memory_size", 5)),
    )


def _build_keyword(
    entry: Any, order: int, synonyms: dict[str, list[str]]
) -> Keyword:
    if not isinstance(entry, dict):
        raise ScriptError(f"keyword #{order} must be an object")
    name = entry.get("keyword")
    if not isinstance(name, str) or not name:
        raise ScriptError(f"keyword #{order} is missing a 'keyword' name")
    name = name.lower()
    rank = entry.get("rank", 0)
    if not isinstance(rank, int):
        raise ScriptError(f"keyword '{name}' has a non-integer rank")

    rules_raw = entry.get("rules", [])
    if not isinstance(rules_raw, list) or not rules_raw:
        raise ScriptError(f"keyword '{name}' must define at least one rule")

    rules = [_build_rule(rule_raw, name, synonyms) for rule_raw in rules_raw]

    memory = [str(m) for m in entry.get("memory", []) or []]
    # Any rule may fill a memory template, so its placeholders must fit the
    # rule with the fewest groups.
    if memory:
        min_groups = min(rule.group_count for rule in rules)
        for tmpl in memory:
            _validate_placeholders(tmpl, min_groups, name, "memory")

    return Keyword(keyword=name, rank=rank, order=order, rules=rules, memory=memory)


def _build_rule(
    rule_raw: Any, kw_name: str, synonyms: dict[str, list[str]]
) -> Rule:
    if not isinstance(rule_raw, dict):
        raise ScriptError(f"keyword '{kw_name}' has a non-object rule")
    decomp = rule_raw.get("decomp")
    if not isinstance(decomp, str) or not decomp.strip():
        raise ScriptError(f"keyword '{kw_name}' has a rule with no 'decomp'")

    regex, group_count = _compile_decomp(decomp, synonyms, kw_name)

    delegate = rule_raw.get("delegate")
    reassemb = rule_raw.get("reassemb", [])
    if delegate is not None:
        if not isinstance(delegate, str) or not delegate:
            raise ScriptError(f"keyword '{kw_name}' has an invalid 'delegate'")
        delegate = delegate.lower()
    else:
        if not isinstance(reassemb, list) or not reassemb:
            raise ScriptError(
                f"keyword '{kw_name}' rule '{decomp}' needs 'reassemb' or 'delegate'"
            )
        reassemb = [str(r) for r in reassemb]
        for tmpl in reassemb:
            _validate_placeholders(tmpl, group_count, kw_name, decomp)

    return Rule(
        decomp=decomp,
        regex=regex,
        group_count=group_count,
        reassemb=list(reassemb) if delegate is None else [],
        delegate=delegate,
    )


def _compile_decomp(
    decomp: str, synonyms: dict[str, list[str]], kw_name: str
) -> tuple[re.Pattern[str], int]:
    atoms: list[str] = []
    groups = 0
    for token in decomp.split():
        if token == "*":
            atoms.append(r"(.*)")
            groups += 1
        elif token.startswith("@"):
            cls = token[1:]
            members = synonyms.get(cls)
            if not members:
                raise ScriptError(
                    f"keyword '{kw_name}' references unknown class '@{cls}'"
                )
            alt = "|".join(re.escape(m) for m in members)
            atoms.append(r"\b(" + alt + r")\b")
            groups += 1
        else:
            atoms.append(r"\b" + re.escape(token) + r"\b")
    pattern = r"\s*".join(atoms)
    return re.compile(pattern, re.IGNORECASE), groups


def _validate_placeholders(
    template: str, group_count: int, kw_name: str, decomp: str
) -> None:
    for match in _PLACEHOLDER.finditer(template):
        n = int(match.group(1))
        if n < 1 or n > group_count:
            raise ScriptError(
                f"keyword '{kw_name}' rule '{decomp}': placeholder ({n}) out of "
                f"range (rule has {group_count} group(s))"
            )
