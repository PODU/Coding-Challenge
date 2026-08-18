import json

import pytest

from reviewagent.github import GitHubClient

STRIPE_PATCH = (
    "@@ -1,3 +1,4 @@\n"
    " import os\n"
    "+STRIPE_KEY = \"sk_live_51Habc123def456ghi789\"\n"
    " DEBUG = False\n"
)

NESTED_PATCH = (
    "@@ -10,4 +10,9 @@ def calculate_discount(order):\n"
    "+    if order.total > 100:\n"
    "+        if order.customer.is_member:\n"
    "+            if order.customer.years > 5:\n"
    "+                if order.coupon:\n"
    "+                    return order.total / order.item_count\n"
)

TRIVIAL_PATCH = (
    "@@ -1,2 +1,2 @@\n"
    "-# Ths is the readme\n"
    "+# This is the readme\n"
)


def pr_files_payload():
    return [
        {"filename": "app/config.py", "status": "modified",
         "additions": 1, "deletions": 0, "patch": STRIPE_PATCH},
        {"filename": "app/orders.py", "status": "modified",
         "additions": 5, "deletions": 0, "patch": NESTED_PATCH},
        {"filename": "uv.lock", "status": "modified",
         "additions": 400, "deletions": 380, "patch": "@@ huge @@"},
        {"filename": "assets/logo.png", "status": "added",
         "additions": 0, "deletions": 0},  # binary: no patch field
    ]


def pr_meta_payload():
    return {
        "title": "Add refund support and reporting",
        "body": "Implements refunds.",
        "head": {"sha": "abc123"},
    }


class FakeTransport:
    """Records requests; serves canned GitHub API responses."""

    def __init__(self, files=None, meta=None):
        self.files = files if files is not None else pr_files_payload()
        self.meta = meta if meta is not None else pr_meta_payload()
        self.requests = []
        self.comments = []

    def __call__(self, url, method="GET", headers=None, payload=None, timeout=60.0):
        self.requests.append({"url": url, "method": method,
                              "headers": headers or {}, "payload": payload})
        if "/files" in url:
            return self.files if "page=1" in url else []
        if "/comments" in url:
            self.comments.append(payload["body"])
            return {"id": 1, "body": payload["body"]}
        if "/pulls/" in url:
            return self.meta
        raise AssertionError(f"unexpected url {url}")


@pytest.fixture
def transport():
    return FakeTransport()


@pytest.fixture
def gh(transport):
    return GitHubClient("test-token", transport=transport, sleep=lambda _: None)


def make_review_json(issues=None, summary="ok"):
    return json.dumps({
        "summary": summary,
        "issues": issues or [],
        "suggestions": [],
        "verdict": "COMMENT" if issues else "APPROVE",
    })
