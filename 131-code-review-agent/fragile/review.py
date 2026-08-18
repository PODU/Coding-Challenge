#!/usr/bin/env python3
"""Step 1: the fragile script. Under 100 lines, and proud of none of them.

Usage:  python review.py OWNER REPO PR_NUMBER

Known limitations (the whole point of the later steps):
- No retry logic: one 502 from GitHub or the model API kills the run.
- No rate-limit handling: a 429 is a stack trace, not a pause.
- No execution history: once the terminal scrolls, the review is gone.
- No error visibility: failures are visible only to whoever ran it.
- One giant prompt: a 60-file PR blows the context window.
"""
import json
import os
import sys
import urllib.request

GITHUB_TOKEN = os.environ["GITHUB_TOKEN"]        # KeyError if unset. Fragile!
API_KEY = os.environ["ANTHROPIC_API_KEY"]

GUIDELINES = """\
- Never commit secrets, API keys, or credentials.
- Avoid deeply nested conditionals; prefer early returns.
- Guard against division by zero.
"""

SYSTEM = (
    "You are a senior engineer reviewing a pull request. Report issues "
    "tagged CRITICAL, MAJOR, or MINOR with file names, then a verdict."
)


def gh_json(url):
    req = urllib.request.Request(url, headers={
        "Authorization": f"Bearer {GITHUB_TOKEN}",
        "Accept": "application/vnd.github+json",
        "User-Agent": "fragile-review-script",
    })
    with urllib.request.urlopen(req) as resp:      # no retry, no timeout
        return json.load(resp)


def ask_model(prompt):
    req = urllib.request.Request(
        "https://api.anthropic.com/v1/messages",
        data=json.dumps({
            "model": "claude-sonnet-5",
            "max_tokens": 4096,
            "system": SYSTEM,
            "messages": [{"role": "user", "content": prompt}],
        }).encode(),
        headers={
            "x-api-key": API_KEY,
            "anthropic-version": "2023-06-01",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:      # invalid key -> stack trace
        data = json.load(resp)
    return "".join(b["text"] for b in data["content"] if b["type"] == "text")


def main():
    owner, repo, number = sys.argv[1], sys.argv[2], sys.argv[3]
    files = gh_json(
        f"https://api.github.com/repos/{owner}/{repo}/pulls/{number}/files"
    )
    diffs = []
    for f in files:                                # everything, even uv.lock
        patch = f.get("patch", "")
        if patch:
            diffs.append(f"### {f['filename']}\n```diff\n{patch}\n```")
    prompt = (
        f"Review guidelines:\n{GUIDELINES}\n\nChanged files:\n\n"
        + "\n\n".join(diffs)                       # one giant prompt
    )
    print(ask_model(prompt))                       # terminal only, no comment


if __name__ == "__main__":
    main()
