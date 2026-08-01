"""A tiny tracer shaped like OpenTelemetry's: nested spans with attributes
and events, exportable as dicts or JSONL. It avoids pulling in the real SDK."""

from __future__ import annotations

import time
from contextlib import contextmanager
from dataclasses import dataclass, field
from pathlib import Path
import json


@dataclass
class Span:
    name: str
    attributes: dict = field(default_factory=dict)
    events: list[dict] = field(default_factory=list)
    parent: str | None = None
    start_time: float = 0.0
    end_time: float = 0.0

    def add_event(self, name: str, attributes: dict | None = None) -> None:
        self.events.append(
            {"name": name, "attributes": attributes or {}, "time": time.time()}
        )

    def set_attribute(self, key: str, value: object) -> None:
        self.attributes[key] = value

    def as_dict(self) -> dict:
        return {
            "name": self.name,
            "parent": self.parent,
            "attributes": self.attributes,
            "events": self.events,
            "duration_ms": round((self.end_time - self.start_time) * 1000, 3),
        }


class Tracer:
    def __init__(self) -> None:
        self.finished: list[Span] = []
        self._stack: list[Span] = []

    @contextmanager
    def span(self, name: str, **attributes: object):
        parent = self._stack[-1].name if self._stack else None
        span = Span(name=name, attributes=dict(attributes), parent=parent)
        span.start_time = time.time()
        self._stack.append(span)
        try:
            yield span
        finally:
            span.end_time = time.time()
            self._stack.pop()
            self.finished.append(span)

    def export(self) -> list[dict]:
        return [s.as_dict() for s in self.finished]

    def export_jsonl(self, path: Path | str) -> None:
        lines = "\n".join(json.dumps(s) for s in self.export())
        Path(path).write_text(lines + "\n", encoding="utf-8")
