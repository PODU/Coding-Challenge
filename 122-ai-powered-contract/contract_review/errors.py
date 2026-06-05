"""Typed exceptions for the contract-review pipeline."""

from __future__ import annotations


class ContractReviewError(Exception):
    """Base class for all contract-review errors."""


class MalformedDocumentError(ContractReviewError):
    """Raised when a document is empty, unreadable, encrypted, or image-only."""


class LLMOutputError(ContractReviewError):
    """Raised when an LLM response cannot be parsed/validated into a model."""


class SessionNotFoundError(ContractReviewError):
    """Raised when a review session id cannot be loaded from the store."""


class ReviewDecisionError(ContractReviewError):
    """Raised when a review decision references an unknown/invalid clause."""
