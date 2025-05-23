"""Engine tests: ranking, decomposition/reassembly, delegation, fallback."""

from __future__ import annotations

import json
import subprocess
import sys

from eliza.engine import ElizaEngine
from eliza.script import Script


def test_highest_rank_keyword_wins(engine: ElizaEngine) -> None:
    # 'computer' (rank 50) beats 'i' (rank 1); 'computers' is pre-normalised.
    assert engine.respond("I think computers are great") == "Do computers worry you?"


def test_rank_tie_broken_by_first_appearance() -> None:
    data = {
        "reflections": {},
        "keywords": [
            {"keyword": "xnone", "rules": [{"decomp": "*", "reassemb": ["I see."]}]},
            {"keyword": "alpha", "rank": 1,
             "rules": [{"decomp": "*", "reassemb": ["ALPHA branch."]}]},
            {"keyword": "beta", "rank": 1,
             "rules": [{"decomp": "*", "reassemb": ["BETA branch."]}]},
        ],
    }
    engine = ElizaEngine(Script.loads(json.dumps(data)))
    assert engine.respond("alpha then beta") == "ALPHA branch."
    assert engine.respond("beta then alpha") == "BETA branch."


# ----------------------------------------- 8.4 decomposition + reassembly + reflect
def test_class_decomposition_and_reflection(engine: ElizaEngine) -> None:
    assert engine.respond("I am unhappy") == (
        "I am sorry to hear you are unhappy. "
        "Do you think coming here will help you not to be unhappy?"
    )


def test_round_robin_reassembly_cycling(engine: ElizaEngine) -> None:
    assert engine.respond("It is my fault") == "Your fault?"
    assert engine.respond("It is my fault") == "Why do you say your fault?"


def test_reflection_of_captured_pronouns(engine: ElizaEngine) -> None:
    # 'me' must reflect to 'you'.
    assert engine.respond("You are afraid of me") == (
        "What makes you think I am afraid of you?"
    )


def test_delegation_redirects_to_target_keyword(engine: ElizaEngine) -> None:
    # 'how' delegates to 'what'; isolate 'how' so no higher-rank keyword wins.
    assert engine.respond("How does this work") == "Why do you ask?"


def test_xnone_fallback_cycles(engine: ElizaEngine) -> None:
    assert engine.respond("qwerty asdf") == "I see."
    assert engine.respond("zxcv") == "Please go on."


def test_keyword_present_but_no_decomp_falls_through() -> None:
    # 'remember' (rank 5) has only specific decomps; if none match we should
    # fall through to the next keyword ('i', rank 1) rather than crash.
    engine = ElizaEngine(Script.load_default())
    assert engine.respond("remember when i walk") == "You say you walk?"


# ----------------------------------------------------- 8.8 REPL smoke (subprocess)
def test_repl_smoke_exits_cleanly() -> None:
    proc = subprocess.run(
        [sys.executable, "-m", "eliza"],
        input="bye\n",
        capture_output=True,
        text=True,
        cwd="src",
        timeout=30,
    )
    assert proc.returncode == 0
    assert "ELIZA" in proc.stdout
    assert "How do you do" in proc.stdout  # an initial line
    assert "Goodbye" in proc.stdout  # a final line
