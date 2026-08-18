"""Configuration loaded from environment variables."""

from __future__ import annotations

import os
from dataclasses import dataclass, field


class ConfigError(RuntimeError):
    pass


DEFAULT_GUIDELINES = """\
- Never commit secrets, API keys, or credentials.
- Avoid deeply nested conditionals; prefer early returns.
- Guard against division by zero and unchecked None access.
- Functions should do one thing; flag functions over ~50 lines.
- Prefer descriptive names over abbreviations.
"""


@dataclass
class Config:
    github_token: str = ""
    provider: str = "mock"
    model: str = ""
    api_key: str = ""
    base_url: str = ""
    guidelines: str = DEFAULT_GUIDELINES
    max_file_workers: int = 4
    request_timeout: float = 60.0
    model_timeout: float = 120.0
    max_retries: int = 3
    skip_patterns: list[str] = field(default_factory=list)

    @classmethod
    def from_env(cls, env: dict[str, str] | None = None) -> "Config":
        env = env if env is not None else dict(os.environ)
        return cls(
            github_token=env.get("GITHUB_TOKEN", ""),
            provider=env.get("REVIEW_PROVIDER", "mock").lower(),
            model=env.get("REVIEW_MODEL", ""),
            api_key=env.get("REVIEW_API_KEY", env.get("ANTHROPIC_API_KEY", "")),
            base_url=env.get("REVIEW_BASE_URL", ""),
            guidelines=env.get("REVIEW_GUIDELINES", DEFAULT_GUIDELINES),
            max_file_workers=int(env.get("REVIEW_MAX_WORKERS", "4")),
            request_timeout=float(env.get("REVIEW_HTTP_TIMEOUT", "60")),
            model_timeout=float(env.get("REVIEW_MODEL_TIMEOUT", "120")),
            max_retries=int(env.get("REVIEW_MAX_RETRIES", "3")),
        )

    def require_github(self) -> None:
        if not self.github_token:
            raise ConfigError("GITHUB_TOKEN is not set")
