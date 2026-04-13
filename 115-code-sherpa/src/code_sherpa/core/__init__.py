"""Orchestration layer that ties the pipeline together."""

from __future__ import annotations

from .sherpa import IngestReport, Sherpa

__all__ = ["Sherpa", "IngestReport"]
