import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from coach.sessions import SessionStore  # noqa: E402
from coach.memory import MemoryBank  # noqa: E402
from coach.tools import ToolRegistry  # noqa: E402
from coach.tools.specs import SpecStore, register as register_specs  # noqa: E402
from coach.tools.web_search import MockWebSearcher, register as register_search  # noqa: E402

CHALLENGES = ROOT / "challenges"


@pytest.fixture
def spec_store() -> SpecStore:
    return SpecStore(CHALLENGES)


@pytest.fixture
def registry(spec_store) -> ToolRegistry:
    reg = ToolRegistry()
    register_specs(reg, spec_store)
    register_search(
        reg,
        MockWebSearcher(
            {
                "huffman": [
                    {
                        "title": "Huffman coding",
                        "url": "https://en.wikipedia.org/wiki/Huffman_coding",
                        "snippet": "Optimal prefix codes.",
                    }
                ]
            }
        ),
    )
    return reg


@pytest.fixture
def sessions(tmp_path) -> SessionStore:
    return SessionStore(tmp_path / "sessions")


@pytest.fixture
def memory(tmp_path) -> MemoryBank:
    return MemoryBank(tmp_path / "memory")
