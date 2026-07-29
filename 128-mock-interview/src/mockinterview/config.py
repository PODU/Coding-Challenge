"""Settings read from the environment or a .env file in the working directory."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

from dotenv import load_dotenv

load_dotenv()

ANAM_SESSION_TOKEN_URL = "https://api.anam.ai/v1/auth/session-token"

# Default Anam persona assets (Cara). Override via env if Anam rotates ids.
DEFAULT_AVATAR_ID = os.environ.get(
    "ANAM_AVATAR_ID", "30fa96d0-26c4-4e55-94a0-517025942e18"
)
DEFAULT_LLM_ID = os.environ.get("ANAM_LLM_ID", "0934d97d-0c3a-4f33-91b0-5e136a0ef466")
DEFAULT_VOICE_ID = os.environ.get(
    "ANAM_VOICE_ID", "6bfbe25a-979d-40f3-a92b-5394170af54b"
)
# Second voice so the two interviewer styles sound different.
ALT_VOICE_ID = os.environ.get("ANAM_ALT_VOICE_ID", DEFAULT_VOICE_ID)


@dataclass
class Settings:
    anam_api_key: str | None = field(
        default_factory=lambda: os.environ.get("ANAM_API_KEY")
    )
    anthropic_api_key: str | None = field(
        default_factory=lambda: os.environ.get("ANTHROPIC_API_KEY")
    )
    data_dir: Path = field(
        default_factory=lambda: Path(os.environ.get("MOCKINTERVIEW_DATA_DIR", "data"))
    )
    # Free Anam tier caps conversations at 3 minutes; leave a safety margin.
    max_session_seconds: int = field(
        default_factory=lambda: int(os.environ.get("MAX_SESSION_SECONDS", "170"))
    )
    feedback_model: str = field(
        default_factory=lambda: os.environ.get("FEEDBACK_MODEL", "claude-opus-4-8")
    )

    @property
    def anam_configured(self) -> bool:
        return bool(self.anam_api_key)

    @property
    def anthropic_configured(self) -> bool:
        return bool(self.anthropic_api_key)


def get_settings() -> Settings:
    return Settings()
