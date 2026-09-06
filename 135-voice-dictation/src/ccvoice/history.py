"""Dictation history and statistics, stored in SQLite.

Each row keeps both the raw transcript and the cleaned text. The raw text is what you
search when cleanup removed something, and it lets the pipeline be tuned against real
recordings later.
"""

from __future__ import annotations

import json
import sqlite3
from dataclasses import asdict, dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable, Optional

from .config import default_home
from .ports import DictationRecord

SCHEMA = """
CREATE TABLE IF NOT EXISTS dictations (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp        TEXT    NOT NULL,
    duration_s       REAL    NOT NULL DEFAULT 0,
    model_id         TEXT    NOT NULL DEFAULT '',
    target_app       TEXT    NOT NULL DEFAULT '',
    raw_text         TEXT    NOT NULL DEFAULT '',
    clean_text       TEXT    NOT NULL DEFAULT '',
    word_count       INTEGER NOT NULL DEFAULT 0,
    injection_method TEXT    NOT NULL DEFAULT '',
    tags             TEXT    NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_dictations_ts ON dictations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_dictations_model ON dictations(model_id);
"""


def utcnow() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


@dataclass
class Stats:
    sessions: int = 0
    words: int = 0
    seconds: float = 0.0
    first_at: str = ""
    last_at: str = ""
    longest_s: float = 0.0
    clipboard_fallbacks: int = 0
    by_model: dict[str, int] = field(default_factory=dict)
    by_day: dict[str, int] = field(default_factory=dict)
    by_app: dict[str, int] = field(default_factory=dict)

    @property
    def words_per_session(self) -> float:
        return self.words / self.sessions if self.sessions else 0.0

    @property
    def words_per_minute(self) -> float:
        return self.words / (self.seconds / 60) if self.seconds else 0.0

    @property
    def busiest_day(self) -> tuple[str, int]:
        return max(self.by_day.items(), key=lambda kv: kv[1]) if self.by_day else ("", 0)

    def render(self) -> str:
        if not self.sessions:
            return "No dictations recorded yet."
        day, day_words = self.busiest_day
        lines = [
            f"sessions          {self.sessions}",
            f"words             {self.words}",
            f"speaking time     {self.seconds / 60:.1f} min",
            f"words/session     {self.words_per_session:.1f}",
            f"words/minute      {self.words_per_minute:.1f}",
            f"longest session   {self.longest_s:.1f}s",
            f"first / last      {self.first_at}  ->  {self.last_at}",
            f"busiest day       {day} ({day_words} words)",
            f"clipboard falls   {self.clipboard_fallbacks}",
        ]
        if self.by_model:
            lines.append("by model          " + ", ".join(f"{k}={v}" for k, v in sorted(self.by_model.items())))
        if self.by_app:
            top = sorted(self.by_app.items(), key=lambda kv: -kv[1])[:5]
            lines.append("top apps          " + ", ".join(f"{k or '?'}={v}" for k, v in top))
        return "\n".join(lines)


class HistoryStore:
    def __init__(self, path: Optional[Path] = None):
        self.path = Path(path) if path else default_home() / "history.db"
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._conn = sqlite3.connect(str(self.path), check_same_thread=False)
        self._conn.row_factory = sqlite3.Row
        self._conn.executescript(SCHEMA)
        self._conn.commit()

    def record(self, entry: DictationRecord) -> int:
        entry.timestamp = entry.timestamp or utcnow()
        entry.word_count = entry.word_count or len(entry.clean_text.split())
        cur = self._conn.execute(
            """INSERT INTO dictations
               (timestamp, duration_s, model_id, target_app, raw_text, clean_text,
                word_count, injection_method, tags)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            (entry.timestamp, entry.duration_s, entry.model_id, entry.target_app,
             entry.raw_text, entry.clean_text, entry.word_count, entry.injection_method,
             json.dumps(entry.tags)),
        )
        self._conn.commit()
        entry.id = cur.lastrowid
        return entry.id

    def delete(self, entry_id: int) -> bool:
        cur = self._conn.execute("DELETE FROM dictations WHERE id = ?", (entry_id,))
        self._conn.commit()
        return cur.rowcount > 0

    def clear(self) -> int:
        cur = self._conn.execute("DELETE FROM dictations")
        self._conn.commit()
        return cur.rowcount

    def get(self, entry_id: int) -> Optional[DictationRecord]:
        row = self._conn.execute("SELECT * FROM dictations WHERE id = ?", (entry_id,)).fetchone()
        return _to_record(row) if row else None

    def recent(self, limit: int = 20) -> list[DictationRecord]:
        rows = self._conn.execute(
            "SELECT * FROM dictations ORDER BY timestamp DESC, id DESC LIMIT ?", (limit,)
        ).fetchall()
        return [_to_record(r) for r in rows]

    def search(
        self,
        query: str = "",
        *,
        limit: int = 20,
        since: Optional[str] = None,
        until: Optional[str] = None,
        model_id: str = "",
        app: str = "",
    ) -> list[DictationRecord]:
        """Substring search over both transcripts, with optional date/model/app filters."""
        where, params = [], []
        for term in [t for t in query.split() if t]:
            where.append("(clean_text LIKE ? COLLATE NOCASE OR raw_text LIKE ? COLLATE NOCASE)")
            params += [f"%{term}%", f"%{term}%"]
        if since:
            where.append("timestamp >= ?")
            params.append(since)
        if until:
            where.append("timestamp <= ?")
            params.append(until)
        if model_id:
            where.append("model_id = ?")
            params.append(model_id)
        if app:
            where.append("target_app LIKE ? COLLATE NOCASE")
            params.append(f"%{app}%")
        sql = "SELECT * FROM dictations"
        if where:
            sql += " WHERE " + " AND ".join(where)
        sql += " ORDER BY timestamp DESC, id DESC LIMIT ?"
        params.append(limit)
        return [_to_record(r) for r in self._conn.execute(sql, params).fetchall()]

    def count(self) -> int:
        return self._conn.execute("SELECT COUNT(*) FROM dictations").fetchone()[0]

    def stats(self, since: Optional[str] = None) -> Stats:
        clause, params = ("WHERE timestamp >= ?", [since]) if since else ("", [])
        row = self._conn.execute(
            f"""SELECT COUNT(*) AS sessions, COALESCE(SUM(word_count), 0) AS words,
                       COALESCE(SUM(duration_s), 0) AS seconds, MIN(timestamp) AS first_at,
                       MAX(timestamp) AS last_at, COALESCE(MAX(duration_s), 0) AS longest
                FROM dictations {clause}""",
            params,
        ).fetchone()
        stats = Stats(
            sessions=row["sessions"],
            words=row["words"],
            seconds=row["seconds"],
            first_at=row["first_at"] or "",
            last_at=row["last_at"] or "",
            longest_s=row["longest"],
        )
        stats.clipboard_fallbacks = self._conn.execute(
            f"SELECT COUNT(*) FROM dictations {clause + ' AND' if clause else 'WHERE'} injection_method = 'clipboard'",
            params,
        ).fetchone()[0]
        for key, table in (("model_id", stats.by_model), ("target_app", stats.by_app)):
            for r in self._conn.execute(
                f"SELECT {key} AS k, SUM(word_count) AS w FROM dictations {clause} GROUP BY {key}", params
            ):
                table[r["k"]] = r["w"] or 0
        for r in self._conn.execute(
            f"SELECT substr(timestamp, 1, 10) AS day, SUM(word_count) AS w "
            f"FROM dictations {clause} GROUP BY day ORDER BY day",
            params,
        ):
            stats.by_day[r["day"]] = r["w"] or 0
        return stats

    def export_jsonl(self, target: Path) -> int:
        rows = self.recent(limit=1_000_000)
        with open(target, "w", encoding="utf-8") as fh:
            for record in rows:
                fh.write(json.dumps(asdict(record), ensure_ascii=False) + "\n")
        return len(rows)

    def close(self) -> None:
        self._conn.close()

    def __enter__(self) -> "HistoryStore":
        return self

    def __exit__(self, *exc) -> None:
        self.close()


def _to_record(row: sqlite3.Row) -> DictationRecord:
    return DictationRecord(
        id=row["id"],
        timestamp=row["timestamp"],
        duration_s=row["duration_s"],
        model_id=row["model_id"],
        target_app=row["target_app"],
        raw_text=row["raw_text"],
        clean_text=row["clean_text"],
        word_count=row["word_count"],
        injection_method=row["injection_method"],
        tags=json.loads(row["tags"] or "[]"),
    )


def parse_since(value: str) -> str:
    """Accept things like `7d`, `12h` or `2026-01-01`."""
    value = value.strip().lower()
    if not value:
        return ""
    now = datetime.now(timezone.utc)
    units = {"d": "days", "h": "hours", "m": "minutes", "w": "weeks"}
    if value[-1] in units and value[:-1].isdigit():
        delta = timedelta(**{units[value[-1]]: int(value[:-1])})
        return (now - delta).isoformat(timespec="seconds")
    return value


def format_rows(records: Iterable[DictationRecord], width: int = 72) -> str:
    lines = []
    for record in records:
        stamp = record.timestamp.replace("T", " ")[:19]
        text = record.clean_text.replace("\n", " / ")
        if len(text) > width:
            text = text[: width - 3] + "..."
        lines.append(f"{record.id:>5}  {stamp}  {record.duration_s:5.1f}s  {record.word_count:>4}w  {text}")
    return "\n".join(lines) if lines else "(no matches)"
