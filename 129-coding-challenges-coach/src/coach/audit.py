"""A mock security audit of the source tree and deploy config.

Checks for hardcoded credentials, IAM roles outside a minimal allowlist,
unencrypted state storage, and enabled debug endpoints.

Usage: ``python -m coach.audit [deploy/agent_config.yaml] [src-root]``
"""

from __future__ import annotations

import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

from .tools.specs import _load_yaml

CREDENTIAL_PATTERNS = [
    re.compile(r"AKIA[0-9A-Z]{16}"),  # AWS access key
    re.compile(r"sk-(?:ant-)?[A-Za-z0-9_-]{20,}"),  # Anthropic/OpenAI-style key
    re.compile(r"AIza[0-9A-Za-z_-]{35}"),  # Google API key
    re.compile(
        r"(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*['\"][A-Za-z0-9+/_-]{16,}['\"]"
    ),
]

ALLOWED_ROLES = {
    "roles/aiplatform.user",
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/storage.objectViewer",
}

BROAD_ROLE = re.compile(r"roles/(owner|editor|viewer)$|\*")

SCAN_SUFFIXES = {".py", ".yaml", ".yml", ".json", ".env", ".toml", ".cfg"}


@dataclass
class CheckResult:
    name: str
    passed: bool
    details: list[str] = field(default_factory=list)


@dataclass
class AuditReport:
    checks: list[CheckResult]

    @property
    def passed(self) -> bool:
        return all(c.passed for c in self.checks)

    def render(self) -> str:
        lines = ["Security audit report", "=" * 40]
        for c in self.checks:
            lines.append(f"[{'PASS' if c.passed else 'FAIL'}] {c.name}")
            lines.extend(f"    - {d}" for d in c.details)
        lines.append("=" * 40)
        lines.append("RESULT: " + ("ALL CHECKS PASSED" if self.passed else "AUDIT FAILED"))
        return "\n".join(lines)


def check_static_credentials(paths: list[Path]) -> CheckResult:
    findings: list[str] = []
    for path in paths:
        try:
            text = path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        for i, line in enumerate(text.splitlines(), 1):
            for pattern in CREDENTIAL_PATTERNS:
                if pattern.search(line):
                    findings.append(f"{path.name}:{i} matches {pattern.pattern[:30]}…")
                    break
    return CheckResult(
        "no static credentials",
        passed=not findings,
        details=findings or ["0 credentials found"],
    )


def check_least_privilege(config: dict) -> CheckResult:
    roles = config.get("iam_roles", []) or []
    bad = [
        r
        for r in roles
        if BROAD_ROLE.search(str(r)) or str(r) not in ALLOWED_ROLES
    ]
    details = [f"role not least-privilege: {r}" for r in bad] or [
        f"{len(roles)} roles, all on minimal allowlist"
    ]
    return CheckResult("least-privilege IAM roles", passed=not bad, details=details)


def check_encrypted_state(config: dict) -> CheckResult:
    encryption = str(config.get("state_encryption", "")).lower()
    return CheckResult(
        "encrypted state storage",
        passed=encryption in {"google-managed", "cmek"},
        details=[f"state_encryption={encryption or '<missing>'}"],
    )


def _as_bool(value: object) -> bool:
    return str(value).strip().lower() in {"true", "1", "yes", "on"}


def probe_debug_endpoints(config: dict) -> dict[str, int]:
    """Pretend to probe the debug endpoints: 200 if enabled, else 404."""
    enabled = _as_bool(config.get("debug_endpoints_enabled", False))
    endpoints = ["/debug/vars", "/debug/pprof", "/internal/state"]
    return {ep: (200 if enabled else 404) for ep in endpoints}


def check_debug_endpoints(config: dict) -> CheckResult:
    statuses = probe_debug_endpoints(config)
    bad = {ep: s for ep, s in statuses.items() if s == 200}
    details = [f"{ep} -> {status}" for ep, status in statuses.items()]
    return CheckResult(
        "debug endpoints return 404/401", passed=not bad, details=details
    )


def run_audit(config_path: Path | str, src_root: Path | str) -> AuditReport:
    config_path = Path(config_path)
    config = _load_yaml(config_path.read_text(encoding="utf-8"))
    scan_paths = [
        p
        for p in Path(src_root).rglob("*")
        if p.is_file() and p.suffix in SCAN_SUFFIXES and "__pycache__" not in p.parts
    ]
    scan_paths.append(config_path)
    return AuditReport(
        checks=[
            check_static_credentials(scan_paths),
            check_least_privilege(config),
            check_encrypted_state(config),
            check_debug_endpoints(config),
        ]
    )


def main(argv: list[str] | None = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    root = Path(__file__).resolve().parents[2]
    config = Path(argv[0]) if argv else root / "deploy" / "agent_config.yaml"
    src = Path(argv[1]) if len(argv) > 1 else root / "src"
    report = run_audit(config, src)
    print(report.render())
    return 0 if report.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
