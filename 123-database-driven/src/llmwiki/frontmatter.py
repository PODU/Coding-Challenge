"""YAML frontmatter for wiki pages, without a YAML library.

Frontmatter only ever holds strings and lists of strings, which is simple
enough to read and write by hand.
"""

from __future__ import annotations

from .domain import Frontmatter, WikiPage

_DELIM = "---"

# Always lists, even when empty or holding a single value.
_LIST_FIELDS = {"sources", "links", "tags"}


def _needs_quote(value: str) -> bool:
    if value == "":
        return True
    if value[0] in " \"'>|@`#&*!%-?:[]{}," or value[-1] == " ":
        return True
    return ":" in value or "#" in value


def _dump_scalar(value: str) -> str:
    if _needs_quote(value):
        escaped = value.replace("\\", "\\\\").replace('"', '\\"')
        return f'"{escaped}"'
    return value


def _load_scalar(raw: str) -> str:
    raw = raw.strip()
    if len(raw) >= 2 and raw[0] == raw[-1] and raw[0] in "\"'":
        inner = raw[1:-1]
        if raw[0] == '"':
            inner = inner.replace('\\"', '"').replace("\\\\", "\\")
        return inner
    return raw


def dump_frontmatter(fm: Frontmatter) -> str:
    d = fm.to_dict()
    lines: list[str] = []
    order = [
        "id", "type", "title", "project", "created_at", "updated_at",
        "sources", "links", "tags", "status", "content_sha256",
    ]
    for key in order:
        value = d[key]
        if isinstance(value, list):
            if not value:
                lines.append(f"{key}: []")
            else:
                lines.append(f"{key}:")
                lines.extend(f"  - {_dump_scalar(str(v))}" for v in value)
        else:
            lines.append(f"{key}: {_dump_scalar(str(value))}")
    return "\n".join(lines)


def to_markdown(page: WikiPage) -> str:
    return f"{_DELIM}\n{dump_frontmatter(page.frontmatter)}\n{_DELIM}\n{page.body.rstrip()}\n"


def parse_frontmatter(text: str) -> tuple[dict, str]:
    if not text.startswith(_DELIM):
        return {}, text
    lines = text.splitlines()
    end = next((i for i in range(1, len(lines)) if lines[i].strip() == _DELIM), None)
    if end is None:
        return {}, text

    data: dict = {}
    current_key: str | None = None
    for line in lines[1:end]:
        if not line.strip():
            continue
        if line.lstrip().startswith("- ") and current_key is not None:
            data.setdefault(current_key, [])
            if not isinstance(data[current_key], list):
                data[current_key] = []
            data[current_key].append(_load_scalar(line.lstrip()[2:]))
            continue
        if ":" in line:
            key, _, rest = line.partition(":")
            key = key.strip()
            rest = rest.strip()
            current_key = key
            if rest == "" or rest == "[]":
                data[key] = [] if (rest == "[]" or key in _LIST_FIELDS) else ""
            else:
                data[key] = _load_scalar(rest)
    for key in _LIST_FIELDS:
        if key in data and not isinstance(data[key], list):
            data[key] = [data[key]] if data[key] else []
    body = "\n".join(lines[end + 1:]).strip("\n")
    return data, body


def from_markdown(text: str) -> WikiPage:
    data, body = parse_frontmatter(text)
    return WikiPage(frontmatter=Frontmatter.from_dict(data), body=body)
