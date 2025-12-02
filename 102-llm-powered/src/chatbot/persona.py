"""Personas. ``system_prompt`` is sent as-is in the ``system`` field."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Persona:
    name: str
    system_prompt: str


BUILT_IN: dict[str, Persona] = {
    "assistant": Persona(
        "assistant",
        "You are a concise, helpful terminal assistant. "
        "Prefer short, direct answers.",
    ),
    "pirate": Persona(
        "pirate",
        "You are a salty pirate captain. Answer correctly but always in "
        "pirate dialect. Never break character.",
    ),
    "tutor": Persona(
        "tutor",
        "You are a patient programming tutor. Explain step by step and ask a "
        "follow-up question to check understanding.",
    ),
}
