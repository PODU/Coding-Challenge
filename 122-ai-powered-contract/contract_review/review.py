"""Pausing for human review between analysis and the summary.

``analyse`` saves a ``pending_review`` session and exits. A reviewer then
records decisions with ``apply_decisions``, and ``resume`` marks the session
ready for the summary. Sessions are stored as JSON files by default.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional, Protocol

from .errors import ReviewDecisionError, SessionNotFoundError
from .models import ReviewDecision, ReviewReport, ReviewSession
from .obs import logger


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class Store(Protocol):
    def save(self, session: ReviewSession) -> None: ...

    def load(self, session_id: str) -> ReviewSession: ...

    def exists(self, session_id: str) -> bool: ...

    def list_ids(self) -> list[str]: ...


class JSONFileStore:
    """Stores each session as ``<directory>/<id>.json``."""

    def __init__(self, directory: str | Path) -> None:
        self.directory = Path(directory)

    def _path(self, session_id: str) -> Path:
        safe = session_id.replace("/", "_").replace("\\", "_")
        return self.directory / f"{safe}.json"

    def save(self, session: ReviewSession) -> None:
        self.directory.mkdir(parents=True, exist_ok=True)
        self._path(session.id).write_text(
            session.model_dump_json(indent=2), encoding="utf-8"
        )

    def load(self, session_id: str) -> ReviewSession:
        path = self._path(session_id)
        if not path.exists():
            raise SessionNotFoundError(f"No session {session_id!r} in {self.directory}")
        return ReviewSession.model_validate_json(path.read_text(encoding="utf-8"))

    def exists(self, session_id: str) -> bool:
        return self._path(session_id).exists()

    def list_ids(self) -> list[str]:
        if not self.directory.exists():
            return []
        return sorted(p.stem for p in self.directory.glob("*.json"))


def new_session(
    report: ReviewReport,
    *,
    contract_text: str = "",
    source_path: str = "",
    session_id: Optional[str] = None,
    store: Optional[Store] = None,
) -> ReviewSession:
    """Create a ``pending_review`` session and save it if a store is given."""

    session = ReviewSession(
        id=session_id or uuid.uuid4().hex[:12],
        status="pending_review",
        report=report,
        decisions=[],
        contract_text=contract_text,
        source_path=source_path,
    )
    if store is not None:
        store.save(session)
    return session


def apply_decisions(
    session: ReviewSession,
    decisions: list[ReviewDecision],
    store: Optional[Store] = None,
) -> ReviewSession:
    """Record reviewer decisions. A later decision on a clause replaces an
    earlier one, and unknown clause numbers raise ``ReviewDecisionError``."""

    valid = {c.number for c in session.report.clauses}
    for d in decisions:
        if d.clause_number not in valid:
            raise ReviewDecisionError(
                f"Decision references unknown clause number {d.clause_number}. "
                f"Valid clauses: {sorted(valid)}"
            )

    merged: dict[int, ReviewDecision] = {d.clause_number: d for d in session.decisions}
    for d in decisions:
        merged[d.clause_number] = d

    session.decisions = [merged[n] for n in sorted(merged)]
    session.updated_at = _utcnow()
    if store is not None:
        store.save(session)
    return session


def resume(session: ReviewSession, store: Optional[Store] = None) -> ReviewSession:
    """Mark a paused session as resumed so the summary can be generated.

    Resuming with no decisions counts as approving everything. Resuming a
    session that is already complete does nothing.
    """

    if session.status == "complete":
        logger.warning("resume() called on already-complete session %s; no-op", session.id)
        return session

    if session.status == "pending_review" and not session.decisions:
        logger.info(
            "resume() on session %s with no decisions; treating all clauses as "
            "implicitly approved",
            session.id,
        )

    session.status = "resumed"
    session.updated_at = _utcnow()
    if store is not None:
        store.save(session)
    return session


def load_session(store: Store, session_id: str) -> ReviewSession:
    return store.load(session_id)
