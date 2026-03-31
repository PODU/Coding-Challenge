"""Structural patterns.

  1. rule of three: "X, Y, and Z"
  2. negative parallelism: "not only X but Y", "it's not just X, it's Y"
  3. outline conclusions: "In conclusion,", "To summarise,", ...
  4. false ranges: "from X to Y"
"""

from __future__ import annotations

import re

from .. import lexicons as lex
from ..tokenize import Document
from .base import Detection, Finding

# "fast, reliable, and scalable" / "fast, reliable and scalable"
_RULE_OF_THREE_RE = re.compile(
    r"\b([^\W\d_]+),\s+([^\W\d_]+),?\s+and\s+([^\W\d_]+)\b", re.IGNORECASE
)

_NEG_PARALLEL_RES = (
    re.compile(r"\bnot only\b.+?\bbut\b", re.IGNORECASE | re.DOTALL),
    re.compile(r"\bit'?s not just\b.+?\bit'?s\b", re.IGNORECASE | re.DOTALL),
    re.compile(r"\bisn'?t just\b.+?\bit'?s\b", re.IGNORECASE | re.DOTALL),
    re.compile(r"\bnot just\b[^.;,]+?\bbut also\b", re.IGNORECASE),
)

# "from spreadsheets to symphonies"
_FALSE_RANGE_RE = re.compile(
    r"\bfrom\s+([^\W\d_]+)\s+to\s+([^\W\d_]+)\b", re.IGNORECASE
)

_OUTLINE_RES = [
    (c, re.compile(rf"(?<!\w){re.escape(c)}", re.IGNORECASE))
    for c in lex.OUTLINE_CONCLUSIONS
]

# Words that make "from X to Y" a real measurement, not a rhetorical range.
_NUMERICISH = re.compile(r"^\d|^(?:one|two|three|four|five|nine|ten|here|there|now|then)$",
                         re.IGNORECASE)


class StructuralDetector:
    name = "Structural patterns"
    step = 3
    cap = lex.CAP_STRUCTURAL

    def run(self, doc: Document) -> Detection:
        text = doc.text
        findings: list[Finding] = []

        for m in _RULE_OF_THREE_RE.finditer(text):
            findings.append(
                Finding(
                    pattern="rule_of_three",
                    evidence=m.group(0).strip(),
                    detail="rule of three (grouped triple)",
                    weight=lex.WEIGHT_RULE_OF_THREE,
                )
            )

        for rx in _NEG_PARALLEL_RES:
            m = rx.search(text)
            if m:
                evidence = " ".join(m.group(0).split())
                if len(evidence) > 60:
                    evidence = evidence[:57] + "..."
                findings.append(
                    Finding(
                        pattern="negative_parallelism",
                        evidence=evidence,
                        detail="negative parallelism (overly formal contrast)",
                        weight=lex.WEIGHT_NEGATIVE_PARALLELISM,
                    )
                )
                break  # one hit is enough

        for conclusion, rx in _OUTLINE_RES:
            if rx.search(text):
                findings.append(
                    Finding(
                        pattern="outline_conclusion",
                        evidence=conclusion,
                        detail=f'outline-style conclusion "{conclusion}"',
                        weight=lex.WEIGHT_OUTLINE_CONCLUSION,
                    )
                )
                break

        for m in _FALSE_RANGE_RE.finditer(text):
            a, b = m.group(1), m.group(2)
            if _NUMERICISH.match(a) or _NUMERICISH.match(b):
                continue
            findings.append(
                Finding(
                    pattern="false_range",
                    evidence=m.group(0).strip(),
                    detail="false range (rhetorical 'from X to Y')",
                    weight=lex.WEIGHT_FALSE_RANGE,
                )
            )

        return Detection.build(self.name, self.step, self.cap, findings)
