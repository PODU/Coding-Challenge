"""End-to-end pipeline tests against the ReviewMe-style fixture PR,
entirely offline via FakeTransport + MockProvider."""

from reviewagent.pipeline import run_review
from reviewagent.providers.mock import MockProvider

from conftest import TRIVIAL_PATCH

GUIDELINES = "- No secrets.\n- No deep nesting.\n"


def run(gh, **kwargs):
    return run_review(gh, MockProvider(), owner="o", repo="r", number=1,
                      guidelines=GUIDELINES, **kwargs)


def test_catches_stripe_key_as_critical(gh):
    result = run(gh)
    criticals = [i for i in result.review.issues if i.severity == "CRITICAL"]
    assert criticals, "hardcoded Stripe key must be flagged CRITICAL"
    assert criticals[0].file == "app/config.py"


def test_flags_deep_nesting(gh):
    result = run(gh)
    assert any(i.severity == "MINOR" and i.file == "app/orders.py"
               for i in result.review.issues)


def test_issues_sorted_by_severity(gh):
    result = run(gh)
    ranks = [i.rank for i in result.review.issues]
    assert ranks == sorted(ranks)


def test_lock_and_binary_files_skipped_and_reported(gh):
    result = run(gh)
    skipped_names = [name for name, _ in result.skipped]
    assert "uv.lock" in skipped_names
    assert "assets/logo.png" in skipped_names
    assert "uv.lock" in result.comment  # transparency in the comment
    assert result.reviewed == ["app/config.py", "app/orders.py"]


def test_posts_single_comment(gh, transport):
    result = run(gh)
    assert result.posted
    assert len(transport.comments) == 1
    assert "Automated Code Review" in transport.comments[0]


def test_trivial_pr_posts_nothing(transport, gh):
    transport.files = [{"filename": "README.md", "status": "modified",
                        "additions": 1, "deletions": 1,
                        "patch": TRIVIAL_PATCH}]
    result = run(gh)
    assert not result.posted
    assert not transport.comments
    assert result.review.verdict == "APPROVE"


def test_dry_run_never_posts(gh, transport):
    result = run(gh, dry_run=True)
    assert not result.posted
    assert not transport.comments
    assert result.review.issues  # still found issues


def test_token_usage_accumulated(gh):
    result = run(gh)
    assert result.input_tokens > 0
    assert result.output_tokens > 0


def test_linter_findings_reach_the_prompt(gh):
    class SpyProvider(MockProvider):
        prompts = []

        def complete(self, system, prompt):
            SpyProvider.prompts.append(prompt)
            return super().complete(system, prompt)

    run_review(gh, SpyProvider(), owner="o", repo="r", number=1,
               guidelines=GUIDELINES,
               linter_findings="- app/orders.py:12 [F841] unused variable")
    assert all("F841" in p for p in SpyProvider.prompts)
