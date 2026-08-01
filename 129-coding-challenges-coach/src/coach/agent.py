"""The coaching agent loop.

A single ``turn()`` can take several model rounds: whenever the model asks for
tools, we run them, append the results, and ask again until it answers in text.
"""

from __future__ import annotations

import json
from dataclasses import dataclass

from .costs import CostTracker, pick_model
from .llm import LLMClient, Message
from .memory import MemoryBank, MemoryNote
from .persona import build_system_prompt
from .sessions import Session, SessionStore
from .telemetry import Tracer
from .tools import ToolRegistry

MAX_TOOL_ROUNDS = 8


@dataclass
class TurnResult:
    text: str
    tool_calls_made: list[str]
    model: str
    rounds: int


class CoachAgent:
    def __init__(
        self,
        client: LLMClient,
        tools: ToolRegistry,
        *,
        session: Session,
        session_store: SessionStore | None = None,
        memory: MemoryBank | None = None,
        costs: CostTracker | None = None,
        tracer: Tracer | None = None,
        catalog: list[str] | None = None,
        max_tokens: int = 1024,
    ) -> None:
        self.client = client
        self.tools = tools
        self.session = session
        self.session_store = session_store
        self.memory = memory
        self.costs = costs
        self.tracer = tracer or Tracer()
        self.catalog = catalog or []
        self.max_tokens = max_tokens

    def _system_prompt(self) -> str:
        notes: list[str] = []
        if self.memory is not None:
            notes = [n.text for n in self.memory.recall(self.session.user_id)]
        return build_system_prompt(catalog=self.catalog, memory_notes=notes)

    def turn(self, user_text: str) -> TurnResult:
        model = pick_model(user_text)
        self.session.append(Message(role="user", content=user_text))
        tool_calls_made: list[str] = []
        rounds = 0
        with self.tracer.span(
            "agent.turn", session_id=self.session.id, model=model
        ) as turn_span:
            while True:
                rounds += 1
                if rounds > MAX_TOOL_ROUNDS:
                    text = (
                        "I hit my tool budget for this turn — let's take stock "
                        "of what we have so far."
                    )
                    break
                if self.costs is not None:
                    self.costs.check_cap()
                with self.tracer.span("llm.request", model=model, round=rounds):
                    result = self.client.complete(
                        system=self._system_prompt(),
                        messages=self.session.history,
                        tools=self.tools.specs,
                        model=model,
                        max_tokens=self.max_tokens,
                    )
                if self.costs is not None:
                    self.costs.record(model, result.usage)
                if not result.wants_tools:
                    text = result.text
                    break
                # Once tools are involved, use the capable model for the rest
                # of the turn.
                model = pick_model(user_text, wants_tools_likely=True)
                for call in result.tool_calls:
                    self.session.append(
                        Message(
                            role="assistant",
                            content=json.dumps(
                                {"tool_call": call.name, "arguments": call.arguments}
                            ),
                            tool_call_id=call.id,
                        )
                    )
                    with self.tracer.span("tool.call", tool=call.name) as tool_span:
                        output = self.tools.call(call.name, call.arguments)
                        tool_span.set_attribute("output_bytes", len(output))
                    tool_calls_made.append(call.name)
                    self.session.append(
                        Message(role="tool", content=output, tool_call_id=call.id)
                    )
            turn_span.set_attribute("rounds", rounds)
            turn_span.set_attribute("tools", tool_calls_made)
        self.session.append(Message(role="assistant", content=text))
        if self.memory is not None and text:
            self.memory.remember(
                self.session.user_id,
                MemoryNote(kind="advice", text=text, session_id=self.session.id),
            )
        if self.session_store is not None:
            self.session_store.save(self.session)
        return TurnResult(
            text=text, tool_calls_made=tool_calls_made, model=model, rounds=rounds
        )
