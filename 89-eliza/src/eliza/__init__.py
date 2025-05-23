"""ELIZA with the DOCTOR script, driven by a JSON script file.

Usage::

    from eliza import ElizaEngine, Script
    engine = ElizaEngine(Script.load_default())
    engine.respond("I am unhappy")
"""

from __future__ import annotations

from .engine import ElizaEngine
from .reflect import reflect
from .script import Keyword, Rule, Script, ScriptError

__all__ = ["ElizaEngine", "Script", "ScriptError", "Keyword", "Rule", "reflect"]
__version__ = "1.0.0"
