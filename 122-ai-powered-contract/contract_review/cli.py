"""The contract-review command: analyse, review, resume, summary and ask."""

from __future__ import annotations

import argparse
import sys
from typing import Optional

from .analysis import analyse_all
from .clauses import extract_clauses
from .config import Config
from .errors import ContractReviewError
from .ingest import load_contract
from .llm import LLMClient, make_client
from .models import ReviewDecision, ReviewSession
from .obs import RunRecorder
from .qa import answer_question
from .report import build_report
from .review import (
    JSONFileStore,
    Store,
    apply_decisions,
    new_session,
    resume,
)
from .summary import build_summary, stream_summary


def _parse_int_list(value: Optional[str]) -> list[int]:
    if not value:
        return []
    out: list[int] = []
    for part in value.split(","):
        part = part.strip()
        if part:
            out.append(int(part))
    return out


def _parse_annotations(values: Optional[list[str]]) -> list[tuple[int, str]]:
    out: list[tuple[int, str]] = []
    for raw in values or []:
        if "=" not in raw:
            raise ContractReviewError(
                f"--annotate expects CLAUSE=note, got {raw!r}"
            )
        num, _, note = raw.partition("=")
        out.append((int(num.strip()), note.strip()))
    return out


def _print_report(session: ReviewSession, out) -> None:
    report = session.report
    print(f"Session: {session.id}  (status: {session.status})", file=out)
    print(
        f"Overall risk: {report.overall_risk}  "
        f"(high={report.high_risk_count}, medium={report.medium_risk_count}, "
        f"low={report.low_risk_count})",
        file=out,
    )
    print(f"Clauses identified: {len(report.clauses)}", file=out)
    for note in report.notes:
        print(f"  ! {note}", file=out)
    by_number = {a.clause_number: a for a in report.analyses}
    for clause in report.clauses:
        analysis = by_number.get(clause.number)
        risk = analysis.risk if analysis else "?"
        print(f"  [{clause.number}] ({risk}) {clause.title}", file=out)


def _print_summary(summary, out) -> None:
    print("\n=== Review Summary ===", file=out)
    print(summary.text, file=out)
    if summary.key_terms:
        print("\nKey terms:", file=out)
        for t in summary.key_terms:
            print(f"  - {t}", file=out)
    if summary.outstanding_risks:
        print("\nOutstanding risks:", file=out)
        for r in summary.outstanding_risks:
            print(f"  - {r}", file=out)


def _generate_summary(session: ReviewSession, llm: LLMClient, store: Store, stream: bool):
    summary = build_summary(session.report, session.decisions, llm, stream=stream)
    session.summary = summary
    session.status = "complete"
    store.save(session)
    return summary


def cmd_analyse(args, llm: LLMClient, store: Store, config: Config, out) -> int:
    recorder = RunRecorder(run_id=args.session_id or "analyse")
    doc = load_contract(args.contract)
    clauses = extract_clauses(doc, llm, config, recorder=recorder)
    analyses = analyse_all(clauses, llm, config, recorder=recorder)
    report = build_report(clauses, analyses)
    session = new_session(
        report,
        contract_text=doc.text,
        source_path=doc.source_path,
        session_id=args.session_id,
        store=store,
    )
    recorder.summary(session_id=session.id, clauses=len(clauses))
    _print_report(session, out)
    if args.no_pause:
        resume(session, store=store)
        summary = _generate_summary(session, llm, store, stream=False)
        _print_summary(summary, out)
    else:
        print(
            f"\nPaused for human review. Use: contract-review review --session "
            f"{session.id} ...",
            file=out,
        )
    return 0


def cmd_review(args, llm: LLMClient, store: Store, config: Config, out) -> int:
    session = store.load(args.session)
    decisions: list[ReviewDecision] = []
    for n in _parse_int_list(args.approve):
        decisions.append(ReviewDecision(clause_number=n, decision="approve"))
    for n in _parse_int_list(args.reject):
        decisions.append(ReviewDecision(clause_number=n, decision="reject"))
    for n, note in _parse_annotations(args.annotate):
        decisions.append(ReviewDecision(clause_number=n, decision="annotate", note=note))
    apply_decisions(session, decisions, store=store)
    print(f"Recorded {len(decisions)} decision(s) on session {session.id}.", file=out)
    for d in session.decisions:
        note = f" — {d.note}" if d.note else ""
        print(f"  clause {d.clause_number}: {d.decision}{note}", file=out)
    return 0


def cmd_resume(args, llm: LLMClient, store: Store, config: Config, out) -> int:
    session = store.load(args.session)
    resume(session, store=store)
    summary = _generate_summary(session, llm, store, stream=args.stream)
    _print_summary(summary, out)
    return 0


def cmd_summary(args, llm: LLMClient, store: Store, config: Config, out) -> int:
    session = store.load(args.session)
    if args.stream and session.summary is None:
        print("Streaming summary:\n", file=out)
        for token in stream_summary(session.report, session.decisions, llm):
            print(token, end="", file=out, flush=True)
        print(file=out)
    if session.summary is not None:
        _print_summary(session.summary, out)
    else:
        summary = _generate_summary(session, llm, store, stream=args.stream)
        _print_summary(summary, out)
    return 0


def cmd_ask(args, llm: LLMClient, store: Store, config: Config, out) -> int:
    session = store.load(args.session)
    answer = answer_question(args.question, session.report, llm)
    print(f"Q: {answer.question}", file=out)
    print(f"A: {answer.answer}", file=out)
    print(f"Grounded: {answer.grounded}", file=out)
    if answer.cited_clauses:
        print(f"Cited clauses: {answer.cited_clauses}", file=out)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="contract-review",
        description="AI-powered contract review agent (Coding Challenge #122).",
    )
    parser.add_argument("--store-dir", default=None, help="Directory for session files.")
    sub = parser.add_subparsers(dest="command", required=True)

    p_analyse = sub.add_parser("analyse", help="Ingest + analyse a contract.")
    p_analyse.add_argument("contract", help="Path to contract (.pdf/.txt/.md).")
    p_analyse.add_argument("--session-id", default=None, help="Explicit session id.")
    p_analyse.add_argument(
        "--no-pause",
        action="store_true",
        help="Skip the HITL pause; resume and summarise immediately.",
    )
    p_analyse.set_defaults(func=cmd_analyse)

    p_review = sub.add_parser("review", help="Record approve/reject/annotate decisions.")
    p_review.add_argument("--session", required=True)
    p_review.add_argument("--approve", default=None, help="Comma-separated clause numbers.")
    p_review.add_argument("--reject", default=None, help="Comma-separated clause numbers.")
    p_review.add_argument(
        "--annotate", action="append", default=None, help='CLAUSE="note" (repeatable).'
    )
    p_review.set_defaults(func=cmd_review)

    p_resume = sub.add_parser("resume", help="Resume a session and generate the summary.")
    p_resume.add_argument("--session", required=True)
    p_resume.add_argument("--stream", action="store_true")
    p_resume.set_defaults(func=cmd_resume)

    p_summary = sub.add_parser("summary", help="Show/generate the review summary.")
    p_summary.add_argument("--session", required=True)
    p_summary.add_argument("--stream", action="store_true")
    p_summary.set_defaults(func=cmd_summary)

    p_ask = sub.add_parser("ask", help="Ask a grounded question about the contract.")
    p_ask.add_argument("--session", required=True)
    p_ask.add_argument("question")
    p_ask.set_defaults(func=cmd_ask)

    return parser


def main(
    argv: Optional[list[str]] = None,
    *,
    llm: Optional[LLMClient] = None,
    store: Optional[Store] = None,
    out=None,
) -> int:
    """Run the CLI. Tests pass ``llm`` and ``store`` to avoid the network and
    the real session directory."""

    out = out or sys.stdout
    parser = build_parser()
    args = parser.parse_args(argv)
    config = Config.from_env()
    if args.store_dir:
        config.store_dir = args.store_dir
    if store is None:
        store = JSONFileStore(config.store_dir)
    if llm is None:
        llm = make_client(config)

    try:
        return args.func(args, llm, store, config, out)
    except ContractReviewError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
