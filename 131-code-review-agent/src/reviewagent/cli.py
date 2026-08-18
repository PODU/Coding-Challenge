"""CLI entry point.

    review-agent OWNER REPO PR_NUMBER [--dry-run] [--lint DIR] [--provider P]

The Kestra flows shell out to this too, so scheduling lives in Kestra and
the review logic lives here.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys

from .config import Config, ConfigError
from .github import GitHubClient
from .linter import format_findings, run_ruff
from .pipeline import run_review
from .providers import make_provider


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="review-agent")
    parser.add_argument("owner")
    parser.add_argument("repo")
    parser.add_argument("number", type=int)
    parser.add_argument("--dry-run", action="store_true",
                        help="print the comment instead of posting it")
    parser.add_argument("--provider", default=None,
                        help="override REVIEW_PROVIDER (anthropic|openai|ollama|mock)")
    parser.add_argument("--model", default=None, help="override REVIEW_MODEL")
    parser.add_argument("--lint", metavar="DIR", default=None,
                        help="run ruff over DIR (a checkout of the PR head) "
                             "and feed findings to the model")
    parser.add_argument("--json", action="store_true",
                        help="emit machine-readable result on stdout")
    args = parser.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s",
                        stream=sys.stderr)

    cfg = Config.from_env()
    if args.provider:
        cfg.provider = args.provider
    if args.model:
        cfg.model = args.model
    try:
        cfg.require_github()
    except ConfigError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    linter_findings = ""
    if args.lint:
        findings = run_ruff(args.lint)
        linter_findings = format_findings(findings, repo_prefix=args.lint)
        logging.info("ruff: %d finding(s)", len(findings))

    gh = GitHubClient(cfg.github_token, timeout=cfg.request_timeout,
                      max_retries=cfg.max_retries)
    provider = make_provider(cfg.provider, api_key=cfg.api_key, model=cfg.model,
                             base_url=cfg.base_url, timeout=cfg.model_timeout)

    result = run_review(
        gh, provider,
        owner=args.owner, repo=args.repo, number=args.number,
        guidelines=cfg.guidelines, max_workers=cfg.max_file_workers,
        linter_findings=linter_findings, dry_run=args.dry_run,
    )

    if args.json:
        print(json.dumps({
            "posted": result.posted,
            "verdict": result.review.verdict,
            "issues": len(result.review.issues),
            "reviewed": result.reviewed,
            "skipped": result.skipped,
            "input_tokens": result.input_tokens,
            "output_tokens": result.output_tokens,
        }))
    else:
        print(result.comment)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
