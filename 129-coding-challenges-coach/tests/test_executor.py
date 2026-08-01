"""Steps 3 & 4 — sandboxed execution, timeouts, output caps, test running."""

import pytest

from coach.tools.executor import LocalCodeExecutor, language_available
from coach.tools.test_runner import run_tests


@pytest.fixture(scope="module")
def sandbox():
    return LocalCodeExecutor(timeout_s=15.0)


def test_python_stdout_and_exit_code(sandbox):
    result = sandbox.execute("print('hello coach')", "python")
    assert result.stdout.strip() == "hello coach"
    assert result.exit_code == 0
    assert not result.timed_out and not result.truncated


def test_python_stderr_and_failure(sandbox):
    result = sandbox.execute("import sys; sys.exit('boom')", "python")
    assert "boom" in result.stderr
    assert result.exit_code == 1


def test_unsupported_language(sandbox):
    result = sandbox.execute("say hi", "cobol")
    assert result.exit_code == 2
    assert "unsupported language" in result.stderr


def test_timeout_kills_run():
    fast = LocalCodeExecutor(timeout_s=1.0)
    result = fast.execute("import time; time.sleep(30)", "python")
    assert result.timed_out
    assert result.exit_code == 124
    assert result.duration_s < 10


def test_output_cap_truncates():
    capped = LocalCodeExecutor(output_cap_bytes=1024)
    result = capped.execute("print('x' * 100_000)", "python")
    assert result.truncated
    assert len(result.stdout.encode()) <= 1024


def test_isolated_fresh_workdir(sandbox):
    # A file written by one run must not exist in the next run's sandbox.
    sandbox.execute("open('marker.txt', 'w').write('here')", "python")
    result = sandbox.execute(
        "import os; print(os.path.exists('marker.txt'))", "python"
    )
    assert result.stdout.strip() == "False"


@pytest.mark.skipif(not language_available("javascript"), reason="node not installed")
def test_javascript(sandbox):
    result = sandbox.execute("console.log(6 * 7)", "javascript")
    assert result.stdout.strip() == "42"
    assert result.exit_code == 0


@pytest.mark.skipif(not language_available("go"), reason="go not installed")
def test_go():
    # go run cold-compiles; give it a generous timeout of its own
    code = 'package main\nimport "fmt"\nfunc main() { fmt.Println("go ok") }\n'
    result = LocalCodeExecutor(timeout_s=120.0).execute(code, "go")
    assert result.stdout.strip() == "go ok"
    assert result.exit_code == 0


def test_run_tests_pass(sandbox):
    out = run_tests(
        sandbox,
        solution="def add(a, b):\n    return a + b\n",
        tests="from solution import add\nassert add(2, 3) == 5\nprint('ok')\n",
        language="python",
    )
    assert out["passed"] is True
    assert "ok" in out["stdout"]


def test_run_tests_fail_reports_details(sandbox):
    out = run_tests(
        sandbox,
        solution="def add(a, b):\n    return a - b\n",
        tests="from solution import add\nassert add(2, 3) == 5\n",
        language="python",
    )
    assert out["passed"] is False
    assert "AssertionError" in out["stderr"]
