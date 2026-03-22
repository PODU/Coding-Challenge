"""Agentic-loop integration tests, fully driven by the scripted MockLLMClient.

These assert the side effects on the workspace and the structure of the
conversation the loop builds -- never touching the network.
"""

from __future__ import annotations

import pytest

from agent.errors import MaxIterationsError
from agent.loop import Agent
from conftest import (
    MockLLMClient,
    end_turn_response,
    tool_use_block,
    tool_use_response,
)


def _agent(mock, registry, workspace, **kw):
    return Agent(llm=mock, registry=registry, workspace=workspace, **kw)


def test_single_tool_round_trip_writes_file(workspace, deny_registry):
    mock = MockLLMClient(
        [
            tool_use_response(
                tool_use_block(
                    "write_file",
                    {"path": "greeting.txt", "content": "hello"},
                    block_id="toolu_1",
                )
            ),
            end_turn_response("Done. I created greeting.txt."),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    final = agent.run("Create greeting.txt")

    assert (workspace.root / "greeting.txt").read_text() == "hello"
    assert "greeting.txt" in final
    assert len(mock.calls) == 2  # tool turn + finishing turn


def test_read_then_write_edit_flow(workspace, deny_registry):
    (workspace.root / "data.txt").write_text("version 1")
    mock = MockLLMClient(
        [
            tool_use_response(
                tool_use_block("read_file", {"path": "data.txt"}, block_id="t1")
            ),
            tool_use_response(
                tool_use_block(
                    "write_file",
                    {"path": "data.txt", "content": "version 2"},
                    block_id="t2",
                )
            ),
            end_turn_response("Edited data.txt to version 2."),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    final = agent.run("Bump the version in data.txt")

    assert (workspace.root / "data.txt").read_text() == "version 2"
    assert "version 2" in final


def test_conversation_state_is_well_formed(workspace, deny_registry):
    mock = MockLLMClient(
        [
            tool_use_response(
                tool_use_block(
                    "write_file",
                    {"path": "f.txt", "content": "x"},
                    block_id="toolu_42",
                )
            ),
            end_turn_response("done"),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    agent.run("write f.txt")

    # The second call's message history is what we inspect.
    second_call_messages = mock.calls[1]["messages"]

    # [0] original user task, [1] assistant tool_use turn, [2] user tool_result turn.
    assert second_call_messages[0]["role"] == "user"
    assert second_call_messages[1]["role"] == "assistant"
    assistant_blocks = second_call_messages[1]["content"]
    assert assistant_blocks[0]["type"] == "tool_use"
    assert assistant_blocks[0]["id"] == "toolu_42"

    user_turn = second_call_messages[2]
    assert user_turn["role"] == "user"
    # All results for the turn live in ONE user message.
    results = user_turn["content"]
    assert isinstance(results, list) and len(results) == 1
    result = results[0]
    assert result["type"] == "tool_result"
    assert result["tool_use_id"] == "toolu_42"  # matches the tool_use id
    assert result["is_error"] is False


def test_tool_error_recovery_feeds_is_error_back(workspace, deny_registry):
    mock = MockLLMClient(
        [
            # First, the model targets a missing file -> recoverable error.
            tool_use_response(
                tool_use_block("read_file", {"path": "missing.txt"}, block_id="e1")
            ),
            # Then it corrects course by creating the file.
            tool_use_response(
                tool_use_block(
                    "write_file",
                    {"path": "missing.txt", "content": "now exists"},
                    block_id="e2",
                )
            ),
            end_turn_response("Recovered and wrote the file."),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    agent.run("read missing.txt, then create it")

    # The second call should carry the error result the model recovers from.
    second_call_messages = mock.calls[1]["messages"]
    error_result = second_call_messages[2]["content"][0]
    assert error_result["is_error"] is True
    assert "file not found" in error_result["content"]

    assert (workspace.root / "missing.txt").read_text() == "now exists"


def test_max_iterations_guard(workspace, deny_registry):
    # Always returns tool_use -> the loop never terminates on its own.
    looping = [
        tool_use_response(
            tool_use_block("list_files", {}, block_id=f"loop{i}")
        )
        for i in range(10)
    ]
    mock = MockLLMClient(looping)
    agent = _agent(mock, deny_registry, workspace, max_iterations=3)

    with pytest.raises(MaxIterationsError):
        agent.run("loop forever")

    assert len(mock.calls) == 3  # exactly max_iterations LLM calls


def test_sandbox_enforced_end_to_end(workspace, deny_registry, tmp_path):
    escape_target = tmp_path.parent / "escape.txt"
    mock = MockLLMClient(
        [
            tool_use_response(
                tool_use_block(
                    "write_file",
                    {"path": "../escape.txt", "content": "pwned"},
                    block_id="s1",
                )
            ),
            end_turn_response("I could not write outside the workspace."),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    agent.run("try to escape")

    # Nothing was written outside the sandbox...
    assert not escape_target.exists()
    # ...and the model received a sandbox-violation error result.
    second_call_messages = mock.calls[1]["messages"]
    result = second_call_messages[2]["content"][0]
    assert result["is_error"] is True
    assert "escapes workspace" in result["content"]


def test_text_plus_tool_use_in_same_turn(workspace, deny_registry):
    from conftest import text_block

    mock = MockLLMClient(
        [
            tool_use_response(
                text_block("I'll create the file now."),
                tool_use_block(
                    "write_file",
                    {"path": "z.txt", "content": "z"},
                    block_id="mix1",
                ),
            ),
            end_turn_response("Created z.txt."),
        ]
    )
    agent = _agent(mock, deny_registry, workspace)
    agent.run("make z.txt")
    assert (workspace.root / "z.txt").read_text() == "z"


def test_run_command_through_loop_in_allow_mode(workspace, allow_registry):
    mock = MockLLMClient(
        [
            tool_use_response(
                tool_use_block(
                    "run_command",
                    {"command": 'python -c "print(2 + 2)"'},
                    block_id="cmd1",
                )
            ),
            end_turn_response("The command printed 4."),
        ]
    )
    agent = _agent(mock, allow_registry, workspace)
    agent.run("run a command")

    result = mock.calls[1]["messages"][2]["content"][0]
    assert result["is_error"] is False
    assert "4" in result["content"]
