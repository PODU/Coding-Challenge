"""Per-stage log records for a pipeline run.

Each record is a plain dict (run id, stage name, counts, timings), which is
also logged. Keeping them as dicts lets tests check them directly.
"""

from __future__ import annotations

import logging
import time
from contextlib import contextmanager
from dataclasses import dataclass, field
from typing import Any, Iterator

logger = logging.getLogger("contract_review")


@dataclass
class RunRecorder:
    run_id: str = "run"
    records: list[dict[str, Any]] = field(default_factory=list)

    def emit(self, stage: str, **fields: Any) -> dict[str, Any]:
        record = {"run_id": self.run_id, "stage": stage, **fields}
        self.records.append(record)
        logger.info("stage=%s %s", stage, fields)
        return record

    @contextmanager
    def timed(self, stage: str, **fields: Any) -> Iterator[dict[str, Any]]:
        """Emit a record for ``stage`` with ``elapsed_ms`` once the block exits.

        Fields set on the yielded dict are added to the record.
        """

        start = time.perf_counter()
        carrier: dict[str, Any] = {}
        try:
            yield carrier
        finally:
            elapsed_ms = round((time.perf_counter() - start) * 1000, 3)
            self.emit(stage, elapsed_ms=elapsed_ms, **{**fields, **carrier})

    def summary(self, **fields: Any) -> dict[str, Any]:
        return self.emit("run_summary", stages=len(self.records), **fields)
