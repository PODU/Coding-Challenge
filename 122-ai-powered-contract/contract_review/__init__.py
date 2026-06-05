"""Contract review with an LLM.

Load a contract, split it into clauses, rate each clause's risk, pause for a
human to review the findings, then write a summary or answer questions about
it. The model sits behind ``LLMClient`` so tests can run against ``MockLLM``.
"""

from __future__ import annotations

from .analysis import analyse_all, analyse_clause
from .chunking import chunk_document
from .clauses import extract_clauses
from .config import Config
from .errors import (
    ContractReviewError,
    LLMOutputError,
    MalformedDocumentError,
    ReviewDecisionError,
    SessionNotFoundError,
)
from .ingest import estimate_tokens, load_contract, load_text, normalise_text
from .llm import ClaudeClient, LLMClient, MockLLM, make_client
from .models import (
    Answer,
    Chunk,
    Clause,
    ClauseAnalysis,
    ContractDocument,
    ReviewDecision,
    ReviewReport,
    ReviewSession,
    Summary,
)
from .qa import answer_question, select_relevant
from .report import build_report
from .review import (
    JSONFileStore,
    Store,
    apply_decisions,
    load_session,
    new_session,
    resume,
)
from .summary import build_summary, stream_summary

__version__ = "0.1.0"

__all__ = [
    "__version__",
    # config / llm
    "Config",
    "LLMClient",
    "ClaudeClient",
    "MockLLM",
    "make_client",
    # models
    "ContractDocument",
    "Chunk",
    "Clause",
    "ClauseAnalysis",
    "ReviewDecision",
    "ReviewReport",
    "ReviewSession",
    "Summary",
    "Answer",
    # ingest / chunk
    "load_contract",
    "load_text",
    "normalise_text",
    "estimate_tokens",
    "chunk_document",
    # pipeline
    "extract_clauses",
    "analyse_clause",
    "analyse_all",
    "build_report",
    "build_summary",
    "stream_summary",
    "answer_question",
    "select_relevant",
    # review
    "Store",
    "JSONFileStore",
    "new_session",
    "apply_decisions",
    "resume",
    "load_session",
    # errors
    "ContractReviewError",
    "MalformedDocumentError",
    "LLMOutputError",
    "SessionNotFoundError",
    "ReviewDecisionError",
]
