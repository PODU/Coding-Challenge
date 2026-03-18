"""SQLite storage for agents and their execution history."""

from __future__ import annotations

import sqlite3
import threading
from datetime import datetime, timezone
from typing import List, Optional

from ..models import Agent, Execution

_SCHEMA = """
CREATE TABLE IF NOT EXISTS agents (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL UNIQUE,
    task          TEXT NOT NULL,
    system_prompt TEXT,
    cron          TEXT NOT NULL,
    timeout_s     REAL NOT NULL,
    max_retries   INTEGER NOT NULL,
    email_to      TEXT,
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS executions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id        INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    agent_name      TEXT NOT NULL,
    started_at      TEXT NOT NULL,
    finished_at     TEXT NOT NULL,
    success         INTEGER NOT NULL,
    response_text   TEXT,
    error           TEXT,
    attempts        INTEGER NOT NULL DEFAULT 0,
    email_delivered INTEGER NOT NULL DEFAULT 0,
    email_error     TEXT
);

CREATE INDEX IF NOT EXISTS ix_exec_agent ON executions(agent_id, id);
"""


class DuplicateAgentError(ValueError):
    """Raised when creating an agent whose name already exists."""


class AgentNotFoundError(KeyError):
    """Raised when an agent name/id does not exist."""


def _parse_dt(value: Optional[str]) -> Optional[datetime]:
    if value is None:
        return None
    dt = datetime.fromisoformat(value)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


class AgentRepository:
    """Thread-safe so the scheduler's worker threads can share it."""

    def __init__(self, db_path: str = ":memory:"):
        self._db_path = db_path
        # check_same_thread=False + a lock so the ThreadPoolExecutor workers can
        # persist results concurrently through one connection.
        self._conn = sqlite3.connect(db_path, check_same_thread=False)
        self._conn.row_factory = sqlite3.Row
        self._conn.execute("PRAGMA foreign_keys = ON")
        self._lock = threading.Lock()
        self._init_schema()

    def _init_schema(self) -> None:
        with self._lock:
            self._conn.executescript(_SCHEMA)
            self._conn.commit()

    def close(self) -> None:
        with self._lock:
            self._conn.close()

    def __enter__(self) -> "AgentRepository":
        return self

    def __exit__(self, *exc) -> None:
        self.close()

    def create_agent(self, agent: Agent) -> Agent:
        now = datetime.now(timezone.utc).isoformat()
        with self._lock:
            try:
                cur = self._conn.execute(
                    """INSERT INTO agents
                       (name, task, system_prompt, cron, timeout_s, max_retries,
                        email_to, enabled, created_at, updated_at)
                       VALUES (?,?,?,?,?,?,?,?,?,?)""",
                    (
                        agent.name,
                        agent.task,
                        agent.system_prompt,
                        agent.cron,
                        agent.timeout_s,
                        agent.max_retries,
                        agent.email_to,
                        1 if agent.enabled else 0,
                        now,
                        now,
                    ),
                )
                self._conn.commit()
            except sqlite3.IntegrityError as exc:
                raise DuplicateAgentError(f"agent {agent.name!r} already exists") from exc
            agent.id = cur.lastrowid
            agent.created_at = _parse_dt(now)
            agent.updated_at = _parse_dt(now)
            return agent

    def get_agent(self, name: str) -> Agent:
        with self._lock:
            row = self._conn.execute(
                "SELECT * FROM agents WHERE name = ?", (name,)
            ).fetchone()
        if row is None:
            raise AgentNotFoundError(name)
        return self._row_to_agent(row)

    def find_agent(self, name: str) -> Optional[Agent]:
        try:
            return self.get_agent(name)
        except AgentNotFoundError:
            return None

    def list_agents(self, *, enabled_only: bool = False) -> List[Agent]:
        sql = "SELECT * FROM agents"
        if enabled_only:
            sql += " WHERE enabled = 1"
        sql += " ORDER BY name"
        with self._lock:
            rows = self._conn.execute(sql).fetchall()
        return [self._row_to_agent(r) for r in rows]

    def update_agent(self, agent: Agent) -> Agent:
        now = datetime.now(timezone.utc).isoformat()
        with self._lock:
            cur = self._conn.execute(
                """UPDATE agents SET
                       task=?, system_prompt=?, cron=?, timeout_s=?, max_retries=?,
                       email_to=?, enabled=?, updated_at=?
                   WHERE name=?""",
                (
                    agent.task,
                    agent.system_prompt,
                    agent.cron,
                    agent.timeout_s,
                    agent.max_retries,
                    agent.email_to,
                    1 if agent.enabled else 0,
                    now,
                    agent.name,
                ),
            )
            self._conn.commit()
        if cur.rowcount == 0:
            raise AgentNotFoundError(agent.name)
        return self.get_agent(agent.name)

    def set_enabled(self, name: str, enabled: bool) -> Agent:
        agent = self.get_agent(name)
        agent.enabled = enabled
        return self.update_agent(agent)

    def delete_agent(self, name: str) -> None:
        with self._lock:
            cur = self._conn.execute("DELETE FROM agents WHERE name = ?", (name,))
            self._conn.commit()
        if cur.rowcount == 0:
            raise AgentNotFoundError(name)

    def save_execution(self, execution: Execution) -> Execution:
        with self._lock:
            cur = self._conn.execute(
                """INSERT INTO executions
                   (agent_id, agent_name, started_at, finished_at, success,
                    response_text, error, attempts, email_delivered, email_error)
                   VALUES (?,?,?,?,?,?,?,?,?,?)""",
                (
                    execution.agent_id,
                    execution.agent_name,
                    execution.started_at.isoformat(),
                    execution.finished_at.isoformat(),
                    1 if execution.success else 0,
                    execution.response_text,
                    execution.error,
                    execution.attempts,
                    1 if execution.email_delivered else 0,
                    execution.email_error,
                ),
            )
            self._conn.commit()
            execution.id = cur.lastrowid
            return execution

    def recent_executions(self, agent_name: str, limit: int = 20) -> List[Execution]:
        """An agent's executions, newest first."""
        with self._lock:
            rows = self._conn.execute(
                "SELECT * FROM executions WHERE agent_name = ? ORDER BY id DESC LIMIT ?",
                (agent_name, limit),
            ).fetchall()
        return [self._row_to_execution(r) for r in rows]

    def all_executions(self, limit: int = 100) -> List[Execution]:
        with self._lock:
            rows = self._conn.execute(
                "SELECT * FROM executions ORDER BY id DESC LIMIT ?", (limit,)
            ).fetchall()
        return [self._row_to_execution(r) for r in rows]

    @staticmethod
    def _row_to_agent(row: sqlite3.Row) -> Agent:
        return Agent(
            id=row["id"],
            name=row["name"],
            task=row["task"],
            system_prompt=row["system_prompt"],
            cron=row["cron"],
            timeout_s=row["timeout_s"],
            max_retries=row["max_retries"],
            email_to=row["email_to"],
            enabled=bool(row["enabled"]),
            created_at=_parse_dt(row["created_at"]),
            updated_at=_parse_dt(row["updated_at"]),
        )

    @staticmethod
    def _row_to_execution(row: sqlite3.Row) -> Execution:
        return Execution(
            id=row["id"],
            agent_id=row["agent_id"],
            agent_name=row["agent_name"],
            started_at=_parse_dt(row["started_at"]),
            finished_at=_parse_dt(row["finished_at"]),
            success=bool(row["success"]),
            response_text=row["response_text"],
            error=row["error"],
            attempts=row["attempts"],
            email_delivered=bool(row["email_delivered"]),
            email_error=row["email_error"],
        )
