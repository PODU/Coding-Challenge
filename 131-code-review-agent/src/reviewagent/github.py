"""GitHub REST client. Tests pass in a fake transport."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable

from . import http
from .retry import with_retries

API = "https://api.github.com"


@dataclass
class ChangedFile:
    filename: str
    status: str
    additions: int
    deletions: int
    patch: str  # GitHub leaves this out for binary and very large files


@dataclass
class PullRequest:
    owner: str
    repo: str
    number: int
    title: str
    body: str
    head_sha: str


class GitHubClient:
    def __init__(
        self,
        token: str,
        *,
        transport: Callable[..., Any] | None = None,
        timeout: float = 60.0,
        max_retries: int = 3,
        sleep: Callable[[float], None] | None = None,
    ):
        self.token = token
        self.timeout = timeout
        self.max_retries = max_retries
        self._transport = transport or http.request_json
        self._sleep_kwargs = {"sleep": sleep} if sleep else {}

    def _headers(self) -> dict[str, str]:
        return {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "cc-review-agent",
        }

    def _get(self, path: str) -> Any:
        return with_retries(
            lambda: self._transport(
                f"{API}{path}", headers=self._headers(), timeout=self.timeout
            ),
            attempts=self.max_retries,
            **self._sleep_kwargs,
        )

    def pull_request(self, owner: str, repo: str, number: int) -> PullRequest:
        data = self._get(f"/repos/{owner}/{repo}/pulls/{number}")
        return PullRequest(
            owner=owner,
            repo=repo,
            number=number,
            title=data.get("title", ""),
            body=data.get("body") or "",
            head_sha=data.get("head", {}).get("sha", ""),
        )

    def changed_files(self, owner: str, repo: str, number: int) -> list[ChangedFile]:
        files: list[ChangedFile] = []
        page = 1
        while True:
            batch = self._get(
                f"/repos/{owner}/{repo}/pulls/{number}/files?per_page=100&page={page}"
            )
            if not batch:
                break
            for f in batch:
                files.append(
                    ChangedFile(
                        filename=f["filename"],
                        status=f.get("status", "modified"),
                        additions=f.get("additions", 0),
                        deletions=f.get("deletions", 0),
                        patch=f.get("patch", "") or "",
                    )
                )
            if len(batch) < 100:
                break
            page += 1
        return files

    def post_comment(self, owner: str, repo: str, number: int, body: str) -> Any:
        return with_retries(
            lambda: self._transport(
                f"{API}/repos/{owner}/{repo}/issues/{number}/comments",
                method="POST",
                headers=self._headers(),
                payload={"body": body},
                timeout=self.timeout,
            ),
            attempts=self.max_retries,
            **self._sleep_kwargs,
        )
