"""Prompts sent to the model.

The team's guidelines are passed in rather than written into the prompt, so
they can change without touching the flows.
"""

from __future__ import annotations

SYSTEM_PROMPT = """\
You are a senior software engineer performing a code review of a pull
request diff. Ground every issue in the diff you are shown; never invent
code that is not present.

Respond with ONLY a JSON object, no markdown fences, in this shape:
{
  "summary": "<one-paragraph overview>",
  "issues": [
    {"severity": "CRITICAL|MAJOR|MINOR", "file": "<path>", "line": <int or null>,
     "title": "<short title>", "detail": "<why it matters and how to fix>"}
  ],
  "suggestions": ["<optional improvement>"],
  "verdict": "APPROVE|REQUEST_CHANGES|COMMENT"
}

Severity meanings:
- CRITICAL: security problems, secrets in code, data loss, crashes.
- MAJOR: correctness bugs, unhandled errors, broken logic.
- MINOR: style, naming, structure, readability.

If a section of tool findings (from a linter) is provided, treat those as
established facts: report them under issues verbatim with their file and
line, do not re-derive or contradict them, and spend your own commentary on
judgment calls the tools cannot make (intent, naming, architecture).

If the change is trivial and correct, return an empty issues list and
verdict APPROVE. Do not manufacture issues to seem useful.
"""


def build_review_prompt(
    *,
    pr_title: str,
    pr_body: str,
    guidelines: str,
    diffs: list[tuple[str, str]],
    linter_findings: str = "",
) -> str:
    """Build the user prompt. ``diffs`` holds (filename, patch) pairs."""
    parts = [
        "## Pull request",
        f"Title: {pr_title}",
        f"Description: {pr_body or '(none)'}",
        "",
        "## Team review guidelines",
        guidelines.strip(),
        "",
    ]
    if linter_findings.strip():
        parts += ["## Tool findings (deterministic — report as fact)",
                  linter_findings.strip(), ""]
    parts.append("## Changed files")
    for filename, patch in diffs:
        parts += [f"### {filename}", "```diff", patch, "```", ""]
    return "\n".join(parts)
