"""Loading contracts from PDF, text or Markdown files.

PDF support needs the optional ``pypdf`` package. Files that are empty,
unreadable, encrypted or have no text layer raise ``MalformedDocumentError``.
"""

from __future__ import annotations

import math
import re
from pathlib import Path

from .errors import MalformedDocumentError
from .models import ContractDocument

# A line that is only a page-number artefact, e.g. "12", "Page 3", "- 4 -".
_PAGE_ARTEFACT = re.compile(r"^\s*(?:-\s*)?(?:page\s+)?\d+(?:\s*-)?\s*$", re.IGNORECASE)
_MULTI_BLANK = re.compile(r"\n{3,}")


def estimate_tokens(text: str) -> int:
    """Rough token count, assuming about 4 characters per token."""

    if not text:
        return 0
    return math.ceil(len(text) / 4)


def normalise_text(text: str) -> str:
    """Normalise line endings and whitespace and drop page-number lines."""

    text = text.replace("\r\n", "\n").replace("\r", "\n")
    lines = []
    for line in text.split("\n"):
        stripped = line.rstrip()
        if _PAGE_ARTEFACT.match(stripped):
            # Blank it rather than drop it, so paragraph breaks survive.
            lines.append("")
            continue
        lines.append(stripped)
    text = "\n".join(lines)
    text = _MULTI_BLANK.sub("\n\n", text)
    return text.strip()


def _load_pdf(path: Path) -> ContractDocument:
    try:
        from pypdf import PdfReader
    except ImportError as exc:  # pragma: no cover - exercised only without pypdf
        raise MalformedDocumentError(
            "PDF support requires the optional 'pypdf' dependency "
            "(install contract-review[pdf])."
        ) from exc

    try:
        reader = PdfReader(str(path))
    except Exception as exc:  # noqa: BLE001 - surface any reader failure uniformly
        raise MalformedDocumentError(f"Could not read PDF {path}: {exc}") from exc

    if getattr(reader, "is_encrypted", False):
        # Many "encrypted" PDFs only have an owner password and open with "".
        try:
            if reader.decrypt("") == 0:  # type: ignore[attr-defined]
                raise MalformedDocumentError(f"PDF {path} is encrypted.")
        except MalformedDocumentError:
            raise
        except Exception as exc:  # noqa: BLE001
            raise MalformedDocumentError(f"PDF {path} is encrypted: {exc}") from exc

    pages = []
    for page in reader.pages:
        try:
            pages.append(page.extract_text() or "")
        except Exception:  # noqa: BLE001 - a bad page should not abort the load
            pages.append("")
    text = normalise_text("\n\n".join(pages))
    if not text.strip():
        raise MalformedDocumentError(
            f"PDF {path} has no extractable text layer (image-only? OCR required)."
        )
    return ContractDocument(
        text=text,
        source_path=str(path),
        page_count=len(reader.pages),
        meta={"format": "pdf"},
    )


def _load_text(path: Path) -> ContractDocument:
    try:
        raw = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        try:
            raw = path.read_text(encoding="latin-1")
        except Exception as exc:  # noqa: BLE001
            raise MalformedDocumentError(f"Could not decode {path}: {exc}") from exc
    text = normalise_text(raw)
    if not text.strip():
        raise MalformedDocumentError(f"Document {path} is empty.")
    suffix = path.suffix.lower().lstrip(".") or "txt"
    return ContractDocument(
        text=text,
        source_path=str(path),
        page_count=None,
        meta={"format": suffix},
    )


def load_contract(path: str | Path) -> ContractDocument:
    """Load a contract from a PDF, text or Markdown file."""

    p = Path(path)
    if not p.exists():
        raise MalformedDocumentError(f"File not found: {p}")
    if p.is_dir():
        raise MalformedDocumentError(f"Expected a file, got a directory: {p}")
    if p.suffix.lower() == ".pdf":
        return _load_pdf(p)
    return _load_text(p)


def load_text(text: str, source_path: str = "<string>") -> ContractDocument:
    """Build a ContractDocument from a string instead of a file."""

    normalised = normalise_text(text)
    if not normalised.strip():
        raise MalformedDocumentError("Provided contract text is empty.")
    return ContractDocument(text=normalised, source_path=source_path, meta={"format": "text"})
