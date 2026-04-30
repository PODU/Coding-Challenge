"""The support bot's turn loop.

Each turn returns either an :class:`Answer` or an :class:`Escalation`, and
emits one ``TurnLog`` either way.
"""

from __future__ import annotations

import time
from datetime import datetime, timezone
from uuid import uuid4

from .config import Config
from .conversation import Conversation
from .escalation import EscalationPolicy
from .llm.base import LLMClient, LLMError
from .models import (
    Answer,
    Citation,
    Escalation,
    EscalationReason,
    HANDOFF_REASONS,
    Hit,
    LLMResponse,
)
from .observability import MetricsSink, TurnLog, estimate_cost_usd
from .prompt_builder import PromptBuilder
from .retriever import Retriever
from .llm.routing import QueryClassifier


class SupportBot:
    def __init__(
        self,
        retriever: Retriever,
        prompt_builder: PromptBuilder,
        llm: LLMClient,
        policy: EscalationPolicy,
        classifier: QueryClassifier,
        metrics: MetricsSink,
        config: Config,
        conversation: Conversation | None = None,
    ) -> None:
        self.retriever = retriever
        self.prompt_builder = prompt_builder
        self.llm = llm
        self.policy = policy
        self.classifier = classifier
        self.metrics = metrics
        self.config = config
        self.conversation = conversation or Conversation()

    def reset(self) -> None:
        self.conversation.reset()

    def _retrieval_query(self, text: str) -> str:
        """Prefix the previous user message so follow-ups like "what port does
        it use?" still retrieve the right chunks."""
        earlier = [
            t.content for t in self.conversation.turns if t.role == "user" and t.content != text
        ]
        if earlier:
            return f"{earlier[-1]} {text}"
        return text

    def turn(self, message: str) -> Answer | Escalation:
        request_id = uuid4().hex[:12]
        started = time.perf_counter()
        text = (message or "").strip()

        def log(
            *,
            escalated: bool,
            reason: EscalationReason | None,
            classification: str | None,
            resp: LLMResponse | None,
        ) -> None:
            model = resp.model if resp else "-"
            in_tok = resp.input_tokens if resp else 0
            out_tok = resp.output_tokens if resp else 0
            self.metrics.emit(
                TurnLog(
                    request_id=request_id,
                    timestamp=datetime.now(timezone.utc).isoformat(),
                    provider=resp.provider if resp else "-",
                    model=model,
                    input_tokens=in_tok,
                    output_tokens=out_tok,
                    latency_ms=(time.perf_counter() - started) * 1000.0,
                    estimated_cost_usd=estimate_cost_usd(model, in_tok, out_tok),
                    fallback_used=resp.fallback_used if resp else False,
                    finish_reason=resp.finish_reason if resp else "-",
                    escalated=escalated,
                    escalation_reason=reason.value if reason else None,
                    classification=classification,
                )
            )

        def escalate(reason: EscalationReason, msg: str, context: list[Hit] | None = None) -> Escalation:
            return Escalation(
                reason=reason,
                message=msg,
                request_id=request_id,
                human_handoff=reason in HANDOFF_REASONS,
                transcript=self.conversation.messages(),
                context=context or [],
            )

        if not text:
            esc = escalate(EscalationReason.EMPTY_INPUT, "Please type a question and I'll help.")
            log(escalated=True, reason=esc.reason, classification=None, resp=None)
            return esc

        self.conversation.add("user", text)

        if self.policy.is_human_request(text):
            esc = escalate(
                EscalationReason.HUMAN_REQUEST,
                "Sure — I'm connecting you with a human support agent.",
            )
            log(escalated=True, reason=esc.reason, classification=None, resp=None)
            return esc

        # Off-topic gets a polite decline, not a human handoff.
        if self.policy.is_off_topic(text, self.retriever.domain_terms):
            esc = escalate(
                EscalationReason.OFF_TOPIC,
                "I can only help with questions about the Coding Challenges platform.",
            )
            log(escalated=True, reason=esc.reason, classification=None, resp=None)
            return esc

        hits = self.retriever.search(self._retrieval_query(text))
        if not hits:
            # Nothing to ground an answer on, so don't bother calling the model.
            esc = escalate(
                EscalationReason.NO_CONTEXT,
                "I couldn't find anything about that in our knowledge base, so I'm "
                "escalating you to a human who can help.",
            )
            log(escalated=True, reason=esc.reason, classification=None, resp=None)
            return esc

        classification = self.classifier.classify(text)

        prompt = self.prompt_builder.build(self.conversation.messages(), hits)
        try:
            resp = self.llm.complete(prompt)
        except LLMError:
            esc = escalate(
                EscalationReason.PROVIDER_ERROR,
                "Our assistant is temporarily unavailable, so I'm escalating you to a human.",
                context=hits,
            )
            log(escalated=True, reason=esc.reason, classification=classification, resp=None)
            return esc

        if self.policy.is_abstention(resp.text):
            self.conversation.consecutive_low_conf += 1
            if self.conversation.consecutive_low_conf >= self.config.escalation_repeat_limit:
                esc = escalate(
                    EscalationReason.REPEAT_LIMIT,
                    "I've been unable to help across several attempts — escalating you to a human.",
                    context=hits,
                )
            else:
                esc = escalate(
                    EscalationReason.LOW_CONFIDENCE,
                    "I don't have a confident, grounded answer for that — escalating to a human.",
                    context=hits,
                )
            log(escalated=True, reason=esc.reason, classification=classification, resp=resp)
            return esc

        self.conversation.consecutive_low_conf = 0
        citations = [
            Citation(title=h.chunk.title, source=h.chunk.source, chunk_id=h.chunk.chunk_id)
            for h in hits
        ]
        self.conversation.add("assistant", resp.text, citations=citations)
        log(escalated=False, reason=None, classification=classification, resp=resp)
        return Answer(text=resp.text, citations=citations, request_id=request_id, model=resp.model)
