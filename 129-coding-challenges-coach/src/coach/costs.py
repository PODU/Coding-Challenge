"""Model choice and cost tracking.

Plain conversation goes to the light model and code or tool work to the capable
one. Once a session's spend reaches the cap, the next model call raises
:class:`CostCapExceeded`.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from .llm import Usage

# (input, output) USD per million tokens. Illustrative numbers only.
PRICES: dict[str, tuple[float, float]] = {
    "light": (0.10, 0.40),
    "capable": (3.00, 15.00),
}

LIGHT_MODEL = "light"
CAPABLE_MODEL = "capable"

CODE_HINTS = ("run", "test", "debug", "error", "code", "fix", "traceback", "compile")


def pick_model(user_text: str, *, wants_tools_likely: bool = False) -> str:
    lowered = user_text.lower()
    if wants_tools_likely or any(hint in lowered for hint in CODE_HINTS):
        return CAPABLE_MODEL
    return LIGHT_MODEL


class CostCapExceeded(Exception):
    def __init__(self, spent_usd: float, cap_usd: float) -> None:
        super().__init__(
            f"session cost cap exceeded: spent ${spent_usd:.4f} of ${cap_usd:.4f}"
        )
        self.spent_usd = spent_usd
        self.cap_usd = cap_usd


@dataclass
class RequestCost:
    model: str
    input_tokens: int
    output_tokens: int
    usd: float


@dataclass
class CostTracker:
    cap_usd: float | None = None
    requests: list[RequestCost] = field(default_factory=list)

    def record(self, model: str, usage: Usage) -> RequestCost:
        in_price, out_price = PRICES.get(model, PRICES[CAPABLE_MODEL])
        usd = (
            usage.input_tokens * in_price + usage.output_tokens * out_price
        ) / 1_000_000
        cost = RequestCost(model, usage.input_tokens, usage.output_tokens, usd)
        self.requests.append(cost)
        return cost

    @property
    def session_usd(self) -> float:
        return sum(r.usd for r in self.requests)

    @property
    def session_tokens(self) -> tuple[int, int]:
        return (
            sum(r.input_tokens for r in self.requests),
            sum(r.output_tokens for r in self.requests),
        )

    def check_cap(self) -> None:
        if self.cap_usd is not None and self.session_usd >= self.cap_usd:
            raise CostCapExceeded(self.session_usd, self.cap_usd)

    def summary(self) -> dict:
        tokens_in, tokens_out = self.session_tokens
        return {
            "requests": len(self.requests),
            "input_tokens": tokens_in,
            "output_tokens": tokens_out,
            "session_usd": round(self.session_usd, 6),
            "cap_usd": self.cap_usd,
        }
