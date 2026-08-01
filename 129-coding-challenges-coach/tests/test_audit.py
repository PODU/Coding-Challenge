"""Mock third-party security audit."""

from pathlib import Path

from coach.audit import main, probe_debug_endpoints, run_audit

ROOT = Path(__file__).resolve().parents[1]
CONFIG = ROOT / "deploy" / "agent_config.yaml"


def test_shipped_config_passes_all_four_checks():
    report = run_audit(CONFIG, ROOT / "src")
    assert [c.passed for c in report.checks] == [True, True, True, True]
    assert report.passed


def test_planted_credential_fails(tmp_path):
    src = tmp_path / "src"
    src.mkdir()
    (src / "leak.py").write_text(
        'api_key = "c2VjcmV0LXNlY3JldC1zZWNyZXQtMTIz"\n', encoding="utf-8"
    )
    report = run_audit(CONFIG, src)
    cred = report.checks[0]
    assert not cred.passed
    assert "leak.py" in cred.details[0]


def test_broad_iam_role_fails(tmp_path):
    config = tmp_path / "cfg.yaml"
    config.write_text(
        "iam_roles:\n  - roles/editor\nstate_encryption: cmek\n"
        "debug_endpoints_enabled: false\n",
        encoding="utf-8",
    )
    report = run_audit(config, tmp_path)
    assert not report.checks[1].passed
    assert "roles/editor" in report.checks[1].details[0]


def test_unencrypted_state_fails(tmp_path):
    config = tmp_path / "cfg.yaml"
    config.write_text(
        "iam_roles:\n  - roles/aiplatform.user\nstate_encryption: none\n"
        "debug_endpoints_enabled: false\n",
        encoding="utf-8",
    )
    report = run_audit(config, tmp_path)
    assert not report.checks[2].passed


def test_debug_endpoints_enabled_fails(tmp_path):
    config = tmp_path / "cfg.yaml"
    config.write_text(
        "iam_roles:\n  - roles/aiplatform.user\nstate_encryption: cmek\n"
        "debug_endpoints_enabled: true\n",
        encoding="utf-8",
    )
    assert probe_debug_endpoints({"debug_endpoints_enabled": True})[
        "/debug/vars"
    ] == 200
    report = run_audit(config, tmp_path)
    assert not report.checks[3].passed


def test_cli_exit_codes(tmp_path, capsys):
    assert main([str(CONFIG), str(ROOT / "src")]) == 0
    assert "ALL CHECKS PASSED" in capsys.readouterr().out
    bad = tmp_path / "bad.yaml"
    bad.write_text("debug_endpoints_enabled: true\n", encoding="utf-8")
    assert main([str(bad), str(tmp_path)]) == 1
