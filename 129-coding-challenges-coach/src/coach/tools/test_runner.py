"""The ``run_tests`` tool.

The user's code is saved as ``solution.<ext>`` next to the test file, which is
then run; a non-zero exit means failure. Python tests import ``solution`` and
JavaScript tests ``require`` it. For Go the solution file is only placed
alongside, not compiled into the test's package.
"""

from __future__ import annotations

from ..llm import ToolSpec
from . import ToolRegistry
from .executor import LocalCodeExecutor

SOLUTION_FILE = {
    "python": "solution.py",
    "javascript": "solution.js",
    "go": "solution.go",
}


def run_tests(
    executor: LocalCodeExecutor, *, solution: str, tests: str, language: str
) -> dict:
    solution_name = SOLUTION_FILE.get(language)
    if solution_name is None:
        return {"error": f"unsupported language: {language}"}
    result = executor.execute(
        tests, language, extra_files={solution_name: solution}
    )
    return {
        "passed": result.exit_code == 0 and not result.timed_out,
        **result.as_dict(),
    }


def register(registry: ToolRegistry, executor: LocalCodeExecutor) -> None:
    registry.register(
        ToolSpec(
            name="run_tests",
            description=(
                "Run a test file against the user's solution in a fresh "
                "sandbox. The solution is saved as solution.<ext> next to the "
                "test file. Returns passed flag, stdout, stderr, exit code."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "solution": {"type": "string", "description": "User's code"},
                    "tests": {"type": "string", "description": "Test file code"},
                    "language": {
                        "type": "string",
                        "enum": list(SOLUTION_FILE),
                    },
                },
                "required": ["solution", "tests", "language"],
            },
        ),
        lambda solution, tests, language: run_tests(
            executor, solution=solution, tests=tests, language=language
        ),
    )
