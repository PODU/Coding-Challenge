"""Quality gates, which pass or fail a workspace.

NullGate: always passes.
MockGate: fails the first N times. The count is saved in the run dir so a
    resume doesn't reset it.
SonarQubeGate: runs sonar-scanner and asks SonarQube for the gate status.
"""

from __future__ import annotations

import base64
import json
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from .config import RunConfig


class GateError(RuntimeError):
    pass


@dataclass
class GateResult:
    passed: bool
    detail: str = ""


class QualityGate(Protocol):
    def evaluate(self, workspace: Path) -> GateResult: ...


class NullGate:
    def evaluate(self, workspace: Path) -> GateResult:
        return GateResult(True, "gate disabled")


class MockGate:
    COUNTER = "gate_calls"

    def __init__(self, fail_times: int, run_dir: Path):
        self.fail_times = fail_times
        self.counter_file = Path(run_dir) / self.COUNTER

    def evaluate(self, workspace: Path) -> GateResult:
        calls = 0
        if self.counter_file.is_file():
            calls = int(self.counter_file.read_text().strip() or 0)
        self.counter_file.write_text(str(calls + 1))
        if calls < self.fail_times:
            return GateResult(False,
                              f"mock gate violation (call {calls + 1} of "
                              f"{self.fail_times} scripted failures): "
                              "code_smells above threshold")
        return GateResult(True, "mock gate passed")


class SonarQubeGate:
    def __init__(self, host: str, token: str, project_key: str,
                 scanner: str = "sonar-scanner", poll_timeout: int = 120):
        self.host = host.rstrip("/")
        self.token = token
        self.project_key = project_key
        self.scanner = scanner
        self.poll_timeout = poll_timeout

    def _api(self, path: str, params: dict) -> dict:
        url = f"{self.host}{path}?{urllib.parse.urlencode(params)}"
        req = urllib.request.Request(url)
        if self.token:
            cred = base64.b64encode(f"{self.token}:".encode()).decode()
            req.add_header("Authorization", f"Basic {cred}")
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except (urllib.error.URLError, json.JSONDecodeError) as e:
            raise GateError(f"SonarQube API error on {path}: {e}") from e

    def evaluate(self, workspace: Path) -> GateResult:
        argv = [self.scanner,
                f"-Dsonar.projectKey={self.project_key}",
                f"-Dsonar.host.url={self.host}",
                "-Dsonar.sources=."]
        if self.token:
            argv.append(f"-Dsonar.token={self.token}")
        proc = subprocess.run(argv, cwd=workspace, capture_output=True, text=True)
        if proc.returncode != 0:
            raise GateError(f"sonar-scanner exit {proc.returncode}: "
                            f"{proc.stderr.strip()[:500]}")
        # Analysis is processed asynchronously; poll until the gate status settles.
        deadline = time.time() + self.poll_timeout
        while True:
            data = self._api("/api/qualitygates/project_status",
                             {"projectKey": self.project_key})
            status = data.get("projectStatus", {}).get("status", "NONE")
            if status in ("OK", "ERROR", "WARN"):
                conditions = data["projectStatus"].get("conditions", [])
                failing = [f"{c.get('metricKey')}={c.get('actualValue')} "
                           f"(threshold {c.get('errorThreshold')})"
                           for c in conditions
                           if c.get("status") == "ERROR"]
                detail = "; ".join(failing) if failing else f"gate {status}"
                return GateResult(status == "OK", detail)
            if time.time() > deadline:
                raise GateError("timed out waiting for SonarQube analysis")
            time.sleep(2)


def make_gate(cfg: RunConfig, run_dir: Path) -> QualityGate:
    if cfg.gate == "none":
        return NullGate()
    if cfg.gate == "mock":
        return MockGate(cfg.gate_fail_times, run_dir)
    if cfg.gate == "sonar":
        return SonarQubeGate(cfg.sonar_host, cfg.sonar_token,
                             cfg.sonar_project_key or f"agentloop-{run_dir.name}",
                             cfg.sonar_scanner)
    raise GateError(f"unknown gate: {cfg.gate}")
