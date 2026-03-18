"""``ai-agents`` command-line interface.

This is the only place the real Anthropic and SMTP/Resend clients are built from
the environment; everything else takes its clients as arguments.
"""

from __future__ import annotations

import argparse
import os
import sys
from typing import Optional

from .config import Settings
from .cron import CronExpression, CronParseError, CronParser
from .cron.expression import describe
from .core import (
    AgentNotFoundError,
    AgentRepository,
    AgentRunner,
    DuplicateAgentError,
    HealthService,
    Scheduler,
)
from .email.client import EmailClient, MockEmailClient, ResendEmailClient, SmtpEmailClient
from .llm.client import AnthropicLLMClient, LLMClient, MockLLMClient
from .models import Agent


def _build_llm(settings: Settings) -> LLMClient:
    if os.environ.get("AI_AGENTS_OFFLINE") == "1" or not os.environ.get("ANTHROPIC_API_KEY"):
        # Lets the CLI work without credentials, e.g. to preview schedules.
        return MockLLMClient(default="(offline mock LLM response)")
    return AnthropicLLMClient(model=settings.llm_model)


def _build_email() -> EmailClient:
    if os.environ.get("RESEND_API_KEY"):
        sender = os.environ.get("EMAIL_FROM", "ai-agents@example.com")
        return ResendEmailClient(os.environ["RESEND_API_KEY"], sender)
    if os.environ.get("SMTP_HOST"):
        return SmtpEmailClient(
            host=os.environ["SMTP_HOST"],
            port=int(os.environ.get("SMTP_PORT", "587")),
            username=os.environ.get("SMTP_USER"),
            password=os.environ.get("SMTP_PASSWORD"),
            sender=os.environ.get("EMAIL_FROM"),
        )
    return MockEmailClient()


def _repo(settings: Settings) -> AgentRepository:
    return AgentRepository(settings.db_path)


def _resolve_cron(value: str, llm: Optional[LLMClient]) -> tuple[str, str]:
    """Accept either a raw cron or natural language; return (cron, explanation)."""
    if CronExpression.is_valid(value):
        return CronExpression.parse(value).raw, describe(value)
    result = CronParser(llm).parse(value)
    return result.cron, result.explanation


def cmd_preview(args, settings: Settings) -> int:
    try:
        result = CronParser(_build_llm(settings)).parse(args.text)
    except CronParseError as exc:
        print(f"Could not interpret schedule: {exc}", file=sys.stderr)
        return 1
    print(result.confirmation)
    return 0


def cmd_create(args, settings: Settings) -> int:
    llm = _build_llm(settings)
    try:
        cron, explanation = _resolve_cron(args.schedule, llm)
    except CronParseError as exc:
        print(f"Invalid schedule: {exc}", file=sys.stderr)
        return 1

    print(f"Schedule interpreted as `{cron}` - {explanation}")
    if not args.yes and not _confirm("Create this agent?"):
        print("Aborted.")
        return 1

    agent = Agent.new(
        name=args.name,
        task=args.task,
        cron=cron,
        system_prompt=args.system_prompt,
        email_to=args.email,
        enabled=not args.disabled,
        settings=settings,
        timeout_s=args.timeout,
        max_retries=args.retries,
    )
    with _repo(settings) as repo:
        try:
            repo.create_agent(agent)
        except DuplicateAgentError as exc:
            print(f"Error: {exc}", file=sys.stderr)
            return 1
    print(f"Created agent {args.name!r}.")
    return 0


def cmd_edit(args, settings: Settings) -> int:
    llm = _build_llm(settings)
    with _repo(settings) as repo:
        try:
            agent = repo.get_agent(args.name)
        except AgentNotFoundError:
            print(f"No such agent: {args.name}", file=sys.stderr)
            return 1
        if args.task is not None:
            agent.task = args.task
        if args.system_prompt is not None:
            agent.system_prompt = args.system_prompt
        if args.email is not None:
            agent.email_to = args.email
        if args.timeout is not None:
            agent.timeout_s = args.timeout
        if args.retries is not None:
            agent.max_retries = args.retries
        if args.schedule is not None:
            try:
                agent.cron, _ = _resolve_cron(args.schedule, llm)
            except CronParseError as exc:
                print(f"Invalid schedule: {exc}", file=sys.stderr)
                return 1
        repo.update_agent(agent)
    print(f"Updated agent {args.name!r}.")
    return 0


def cmd_list(args, settings: Settings) -> int:
    with _repo(settings) as repo:
        agents = repo.list_agents()
    if not agents:
        print("No agents configured.")
        return 0
    for a in agents:
        flag = "enabled" if a.enabled else "disabled"
        print(f"{a.name:20} {a.cron:18} [{flag}]  -> {a.email_to or '(no email)'}")
        print(f"    {describe(a.cron)}")
    return 0


def cmd_delete(args, settings: Settings) -> int:
    with _repo(settings) as repo:
        try:
            repo.delete_agent(args.name)
        except AgentNotFoundError:
            print(f"No such agent: {args.name}", file=sys.stderr)
            return 1
    print(f"Deleted agent {args.name!r}.")
    return 0


def cmd_enable(args, settings: Settings) -> int:
    return _set_enabled(args.name, True, settings)


def cmd_disable(args, settings: Settings) -> int:
    return _set_enabled(args.name, False, settings)


def _set_enabled(name: str, enabled: bool, settings: Settings) -> int:
    with _repo(settings) as repo:
        try:
            repo.set_enabled(name, enabled)
        except AgentNotFoundError:
            print(f"No such agent: {name}", file=sys.stderr)
            return 1
    print(f"{'Enabled' if enabled else 'Disabled'} agent {name!r}.")
    return 0


def cmd_run(args, settings: Settings) -> int:
    llm = _build_llm(settings)
    email = _build_email()
    with _repo(settings) as repo:
        try:
            agent = repo.get_agent(args.name)
        except AgentNotFoundError:
            print(f"No such agent: {args.name}", file=sys.stderr)
            return 1
        execution = AgentRunner(llm, email, repo, settings).run_once(agent)
    status = "SUCCESS" if execution.success else "FAILED"
    print(f"[{status}] {agent.name} (attempts={execution.attempts})")
    if execution.success:
        print(execution.response_text)
        if agent.email_to:
            ed = "delivered" if execution.email_delivered else f"failed ({execution.email_error})"
            print(f"Email to {agent.email_to}: {ed}")
    else:
        print(f"Error: {execution.error}", file=sys.stderr)
    return 0 if execution.success else 1


def cmd_status(args, settings: Settings) -> int:
    with _repo(settings) as repo:
        report = HealthService(repo, settings).report()
        print(
            f"System: {'HEALTHY' if report.healthy else 'UNHEALTHY'} | "
            f"agents={report.total_agents} "
            f"(enabled={report.enabled_agents}, disabled={report.disabled_agents})"
        )
        if report.unhealthy_agents:
            print(f"Unhealthy: {', '.join(report.unhealthy_agents)}")
        for s in report.agents:
            health_flag = "healthy" if s.healthy else "UNHEALTHY"
            print(
                f"  {s.agent_name:20} runs={s.total_runs:<4} "
                f"success_rate={s.success_rate:5.0%} "
                f"consecutive_failures={s.consecutive_failures} [{health_flag}]"
            )
    return 0


def cmd_serve(args, settings: Settings) -> int:  # pragma: no cover - long-running
    llm = _build_llm(settings)
    email = _build_email()
    repo = _repo(settings)
    runner = AgentRunner(llm, email, repo, settings)
    scheduler = Scheduler(repo, runner, settings)
    print(f"Scheduler started (tick={settings.scheduler_tick_s}s). Ctrl-C to stop.")
    try:
        scheduler.run_forever()
    except KeyboardInterrupt:
        print("\nStopping scheduler (draining workers)...")
        scheduler.stop()
        scheduler.shutdown()
    finally:
        repo.close()
    return 0


def _confirm(prompt: str) -> bool:  # pragma: no cover - interactive
    try:
        return input(f"{prompt} [y/N] ").strip().lower() in ("y", "yes")
    except EOFError:
        return False


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="ai-agents", description="AI Agent Scheduling (Challenge #111)")
    sub = p.add_subparsers(dest="command", required=True)

    c = sub.add_parser("create", help="create a new agent")
    c.add_argument("name")
    c.add_argument("--task", required=True)
    c.add_argument("--schedule", required=True, help="cron expr or natural language")
    c.add_argument("--system-prompt", default=None)
    c.add_argument("--email", default=None)
    c.add_argument("--timeout", type=float, default=None)
    c.add_argument("--retries", type=int, default=None)
    c.add_argument("--disabled", action="store_true")
    c.add_argument("-y", "--yes", action="store_true", help="skip confirmation")
    c.set_defaults(func=cmd_create)

    e = sub.add_parser("edit", help="edit an existing agent")
    e.add_argument("name")
    e.add_argument("--task", default=None)
    e.add_argument("--schedule", default=None)
    e.add_argument("--system-prompt", default=None)
    e.add_argument("--email", default=None)
    e.add_argument("--timeout", type=float, default=None)
    e.add_argument("--retries", type=int, default=None)
    e.set_defaults(func=cmd_edit)

    sub.add_parser("list", help="list agents").set_defaults(func=cmd_list)

    d = sub.add_parser("delete", help="delete an agent")
    d.add_argument("name")
    d.set_defaults(func=cmd_delete)

    en = sub.add_parser("enable", help="enable an agent")
    en.add_argument("name")
    en.set_defaults(func=cmd_enable)

    di = sub.add_parser("disable", help="disable an agent")
    di.add_argument("name")
    di.set_defaults(func=cmd_disable)

    r = sub.add_parser("run", help="manually fire an agent now")
    r.add_argument("name")
    r.set_defaults(func=cmd_run)

    pv = sub.add_parser("preview", help="preview NL→cron interpretation")
    pv.add_argument("text")
    pv.set_defaults(func=cmd_preview)

    sub.add_parser("status", help="show system + per-agent health").set_defaults(func=cmd_status)
    sub.add_parser("serve", help="run the scheduler loop").set_defaults(func=cmd_serve)
    return p


def main(argv: Optional[list] = None) -> int:
    args = build_parser().parse_args(argv)
    return args.func(args, Settings.from_env())


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
