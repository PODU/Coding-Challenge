"""An offline stand-in for a model, for tests and demos.

It does a shallow scan of the diff in the prompt for hardcoded secrets,
division by zero and deep nesting, and answers in the same JSON shape a real
model is asked for.
"""

from __future__ import annotations

import json
import re

from . import Completion

_SECRET_RE = re.compile(
    r"(sk_live_[0-9a-zA-Z]+|sk-[0-9a-zA-Z]{20,}|AKIA[0-9A-Z]{16}"
    r"|(?:api_key|apikey|secret|token|password)\s*=\s*[\"'][^\"']{8,}[\"'])",
    re.IGNORECASE,
)
_DIV_RE = re.compile(r"/\s*(?:len\(|count|total|n\b|num_)")


class MockProvider:
    def complete(self, system: str, prompt: str) -> Completion:
        issues = []
        current_file = ""
        in_diff = False
        depth_flagged: set[str] = set()

        def flag(severity: str, title: str, detail: str) -> None:
            issues.append(
                {
                    "severity": severity,
                    "file": current_file,
                    "line": None,
                    "title": title,
                    "detail": detail,
                }
            )

        for line in prompt.splitlines():
            if line.startswith("### "):
                current_file = line[4:].strip()
                in_diff = False
            elif line.startswith("```diff"):
                in_diff = True
            elif line.startswith("```"):
                in_diff = False
            elif in_diff and line.startswith("+"):
                added = line[1:]
                if _SECRET_RE.search(added):
                    flag(
                        "CRITICAL",
                        "Hardcoded secret committed to source",
                        "Move the credential to an environment "
                        "variable or secret manager and rotate it.",
                    )
                if _DIV_RE.search(added):
                    flag(
                        "MAJOR",
                        "Possible division by zero",
                        "Guard the denominator before dividing.",
                    )
                indent = len(added) - len(added.lstrip())
                if indent >= 16 and current_file not in depth_flagged:
                    depth_flagged.add(current_file)
                    flag(
                        "MINOR",
                        "Deeply nested conditionals",
                        "Refactor with early returns to reduce nesting depth.",
                    )

        if any(i["severity"] in ("CRITICAL", "MAJOR") for i in issues):
            verdict = "REQUEST_CHANGES"
        elif issues:
            verdict = "COMMENT"
        else:
            verdict = "APPROVE"
        if issues:
            summary = f"Mock review: {len(issues)} issue(s) found."
        else:
            summary = "Mock review: no issues found."
        body = {
            "summary": summary,
            "issues": issues,
            "suggestions": [],
            "verdict": verdict,
        }
        text = json.dumps(body)
        return Completion(
            text=text,
            input_tokens=len(prompt) // 4,
            output_tokens=len(text) // 4,
            model="mock",
        )
