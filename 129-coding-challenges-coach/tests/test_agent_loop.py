"""Tool chaining within a single turn, wired through the agent loop."""

import json

from coach.agent import CoachAgent
from coach.llm import ToolCall
from coach.mock_client import MockLLMClient


def make_agent(sessions, script, registry, **kwargs):
    session = sessions.create("dev1")
    client = MockLLMClient(script=script)
    agent = CoachAgent(
        client,
        registry,
        session=session,
        session_store=sessions,
        catalog=["01-wc.md"],
        **kwargs,
    )
    return agent, client


def test_plain_text_turn(sessions, registry):
    agent, client = make_agent(sessions, ["What languages do you enjoy?"], registry)
    result = agent.turn("hi, I want to learn")
    assert result.text == "What languages do you enjoy?"
    assert result.tool_calls_made == []
    assert result.rounds == 1
    # persona + catalog reached the model
    assert "NEVER provide a full solution" in client.calls[0]["system"]
    assert "01-wc.md" in client.calls[0]["system"]
    # tool schemas advertised
    names = [t.name for t in client.calls[0]["tools"]]
    assert "read_challenge_spec" in names and "web_search" in names


def test_tool_chaining_single_turn(sessions, registry):
    script = [
        [ToolCall(id="a", name="read_challenge_spec", arguments={"name": "03-huffman.yaml"})],
        [ToolCall(id="b", name="web_search", arguments={"query": "huffman coding"})],
        "Huffman fits you: it is advanced and about compression.",
    ]
    agent, client = make_agent(sessions, script, registry)
    result = agent.turn("tell me about the compression challenge")
    assert result.tool_calls_made == ["read_challenge_spec", "web_search"]
    assert result.rounds == 3
    assert "Huffman fits you" in result.text
    # second model round saw the first tool's result in history
    round2_history = client.calls[1]["messages"]
    tool_msgs = [m for m in round2_history if m.role == "tool"]
    assert any("Huffman" in m.content for m in tool_msgs)


def test_parallel_tool_calls_one_round(sessions, registry):
    script = [
        [
            ToolCall(id="a", name="list_challenges", arguments={}),
            ToolCall(id="b", name="web_search", arguments={"query": "huffman"}),
        ],
        "Here is what I found.",
    ]
    agent, _ = make_agent(sessions, script, registry)
    result = agent.turn("what can I work on?")
    assert result.tool_calls_made == ["list_challenges", "web_search"]
    assert result.rounds == 2


def test_tool_error_fed_back_to_model(sessions, registry):
    script = [
        [ToolCall(id="a", name="read_challenge_spec", arguments={"name": "missing.md"})],
        "That spec does not exist locally; want me to search the web?",
    ]
    agent, client = make_agent(sessions, script, registry)
    result = agent.turn("read missing.md")
    tool_msgs = [m for m in client.calls[1]["messages"] if m.role == "tool"]
    assert "no such spec" in json.loads(tool_msgs[-1].content)["error"]
    assert "does not exist" in result.text


def test_runaway_tool_loop_is_bounded(sessions, registry):
    script = [
        [ToolCall(id=str(i), name="list_challenges", arguments={})] for i in range(50)
    ]
    agent, _ = make_agent(sessions, script, registry)
    result = agent.turn("loop forever")
    assert result.rounds <= 9
    assert "tool budget" in result.text
