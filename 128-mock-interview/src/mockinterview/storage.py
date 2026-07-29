"""Interview records, stored as one JSON file per interview."""

from __future__ import annotations

import json
import re
import uuid
from datetime import datetime, timezone
from pathlib import Path

_ID_RE = re.compile(r"^[a-f0-9]{12}$")


class Storage:
    def __init__(self, data_dir: Path):
        self._dir = Path(data_dir)
        self._dir.mkdir(parents=True, exist_ok=True)

    def _path(self, interview_id: str) -> Path:
        if not _ID_RE.match(interview_id):
            raise KeyError(interview_id)
        return self._dir / f"{interview_id}.json"

    def create(self, setup: dict, questions: list[dict], system_prompt: str) -> dict:
        record = {
            "id": uuid.uuid4().hex[:12],
            "created_at": datetime.now(timezone.utc).isoformat(),
            "setup": setup,
            "questions": questions,
            "system_prompt": system_prompt,
            "transcript": [],
            "skipped_question_ids": [],
            "ended_early": False,
            "feedback": None,
        }
        self._write(record)
        return record

    def get(self, interview_id: str) -> dict:
        path = self._path(interview_id)
        if not path.exists():
            raise KeyError(interview_id)
        return json.loads(path.read_text(encoding="utf-8"))

    def save(self, record: dict) -> None:
        self._write(record)

    def list_all(self) -> list[dict]:
        records = []
        for path in self._dir.glob("*.json"):
            try:
                records.append(json.loads(path.read_text(encoding="utf-8")))
            except (json.JSONDecodeError, OSError):
                continue
        records.sort(key=lambda r: r.get("created_at", ""), reverse=True)
        return records

    def summaries(self) -> list[dict]:
        out = []
        for r in self.list_all():
            fb = r.get("feedback") or {}
            out.append(
                {
                    "id": r["id"],
                    "created_at": r["created_at"],
                    "setup": r["setup"],
                    "num_questions": len(r.get("questions", [])),
                    "has_transcript": bool(r.get("transcript")),
                    "average_score": fb.get("average_score"),
                }
            )
        return out

    def _write(self, record: dict) -> None:
        path = self._dir / f"{record['id']}.json"
        tmp = path.with_suffix(".tmp")
        tmp.write_text(json.dumps(record, indent=2), encoding="utf-8")
        tmp.replace(path)
