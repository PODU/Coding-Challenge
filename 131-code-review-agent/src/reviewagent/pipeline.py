"""The whole review: fetch the PR, review each file in parallel, merge the
results and post one comment if there is anything worth saying.
"""

from __future__ import annotations

import logging
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field

from .github import ChangedFile, GitHubClient, PullRequest
from .prompts import SYSTEM_PROMPT, build_review_prompt
from .review import ParseError, Review, format_comment, merge_reviews, parse_review, should_post
from .skiplist import partition

log = logging.getLogger("reviewagent")


@dataclass
class PipelineResult:
    review: Review
    comment: str
    posted: bool
    reviewed: list[str] = field(default_factory=list)
    skipped: list[tuple[str, str]] = field(default_factory=list)
    input_tokens: int = 0
    output_tokens: int = 0


def review_single_file(
    provider,
    pr: PullRequest,
    f: ChangedFile,
    guidelines: str,
    linter_findings: str = "",
) -> tuple[Review, int, int]:
    prompt = build_review_prompt(
        pr_title=pr.title,
        pr_body=pr.body,
        guidelines=guidelines,
        diffs=[(f.filename, f.patch)],
        linter_findings=linter_findings,
    )
    completion = provider.complete(SYSTEM_PROMPT, prompt)
    try:
        review = parse_review(completion.text, default_file=f.filename)
    except ParseError as exc:
        log.warning("unparseable review for %s: %s", f.filename, exc)
        review = Review(summary=f"(model output for {f.filename} was unparseable)")
    return review, completion.input_tokens, completion.output_tokens


def run_review(
    gh: GitHubClient,
    provider,
    *,
    owner: str,
    repo: str,
    number: int,
    guidelines: str,
    max_workers: int = 4,
    linter_findings: str = "",
    skip_patterns: list[str] | None = None,
    dry_run: bool = False,
) -> PipelineResult:
    pr = gh.pull_request(owner, repo, number)
    files = gh.changed_files(owner, repo, number)
    to_review, skipped = partition(files, skip_patterns)
    skipped_pairs = [(f.filename, reason) for f, reason in skipped]
    for name, reason in skipped_pairs:
        log.info("skipping %s: %s", name, reason)

    reviews: list[Review] = []
    input_tokens = output_tokens = 0
    if to_review:
        with ThreadPoolExecutor(max_workers=max(1, max_workers)) as pool:
            futures = [
                pool.submit(
                    review_single_file, provider, pr, f, guidelines, linter_findings
                )
                for f in to_review
            ]
            for fut in futures:
                review, used_in, used_out = fut.result()
                reviews.append(review)
                input_tokens += used_in
                output_tokens += used_out

    merged = merge_reviews(reviews)
    comment = format_comment(
        merged,
        skipped=skipped_pairs,
        reviewed_files=[f.filename for f in to_review],
    )

    posted = False
    if should_post(merged):
        if dry_run:
            log.info("dry run: would post comment (%d issues)", len(merged.issues))
        else:
            gh.post_comment(owner, repo, number, comment)
            posted = True
    else:
        log.info(
            "not posting: no issues found for %s/%s#%d (avoiding noise)",
            owner, repo, number,
        )

    return PipelineResult(
        review=merged,
        comment=comment,
        posted=posted,
        reviewed=[f.filename for f in to_review],
        skipped=skipped_pairs,
        input_tokens=input_tokens,
        output_tokens=output_tokens,
    )
