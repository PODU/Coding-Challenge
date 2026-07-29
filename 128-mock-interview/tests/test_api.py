import pytest
from fastapi.testclient import TestClient

from mockinterview.anam import MockAnamClient
from mockinterview.app import create_app
from mockinterview.config import Settings
from mockinterview.llm import MockFeedbackClient

LONG_ANSWER = (
    "When I was on the platform team we had an incident during a deploy. My role was "
    "on-call engineer so I had to lead the response. I decided to roll back first and "
    "then I wrote a postmortem. The outcome was we improved our deploy checks."
)


@pytest.fixture()
def client(tmp_path):
    settings = Settings(anam_api_key=None, anthropic_api_key=None, data_dir=tmp_path)
    anam = MockAnamClient()
    app = create_app(settings, anam_client=anam, feedback_client=MockFeedbackClient())
    c = TestClient(app)
    c.anam = anam
    return c


def _create_interview(client, **overrides):
    body = {
        "role": "Software Engineer",
        "seniority": "senior",
        "competencies": ["conflict", "failure"],
        "style": "bar_raiser",
        "num_questions": 2,
    }
    body.update(overrides)
    resp = client.post("/api/interviews", json=body)
    assert resp.status_code == 200
    return resp.json()


def test_config_reports_mock_mode(client):
    cfg = client.get("/api/config").json()
    assert cfg["mock_video"] is True
    assert cfg["mock_feedback"] is True
    assert len(cfg["styles"]) == 2
    assert "conflict" in cfg["competencies"]


def test_create_interview_returns_questions(client):
    data = _create_interview(client)
    assert len(data["questions"]) == 2
    assert all(q["competency"] in ("conflict", "failure") for q in data["questions"])


def test_session_token_uses_persona_config(client):
    data = _create_interview(client)
    resp = client.post("/api/session-token", json={"interview_id": data["id"]})
    assert resp.status_code == 200
    body = resp.json()
    assert body["session_token"].startswith("mock-session-token-")
    assert body["mock"] is True
    # The persona sent to Anam embeds the interview's questions.
    sent = client.anam.requests[-1]
    assert data["questions"][0]["text"] in sent["systemPrompt"]
    assert sent["name"].startswith("Alex")


def test_session_token_unknown_interview_404(client):
    resp = client.post("/api/session-token", json={"interview_id": "nope"})
    assert resp.status_code == 404


def test_transcript_roundtrip_and_feedback(client):
    data = _create_interview(client)
    q1, q2 = data["questions"]
    transcript = [
        {"role": "assistant", "content": f"Welcome. {q1['text']}"},
        {"role": "user", "content": LONG_ANSWER},
        {"role": "assistant", "content": f"Thanks. {q2['text']}"},
        {"role": "user", "content": LONG_ANSWER},
        {"role": "assistant", "content": "That is the end. Thank you."},
    ]
    resp = client.post(
        f"/api/interviews/{data['id']}/transcript",
        json={"transcript": transcript, "skipped_question_ids": [], "ended_early": False},
    )
    assert resp.status_code == 200

    stored = client.get(f"/api/interviews/{data['id']}").json()
    assert len(stored["transcript"]) == 5
    assert "system_prompt" not in stored

    report = client.post(f"/api/interviews/{data['id']}/feedback")
    assert report.status_code == 200
    body = report.json()
    assert len(body["answers"]) == 2
    assert body["average_score"] is not None


def test_feedback_without_transcript_400(client):
    data = _create_interview(client)
    resp = client.post(f"/api/interviews/{data['id']}/feedback")
    assert resp.status_code == 400


def test_skipped_question_not_scored(client):
    data = _create_interview(client)
    q1, q2 = data["questions"]
    transcript = [
        {"role": "assistant", "content": f"Welcome. {q1['text']}"},
        {"role": "user", "content": "I'd like to skip this question."},
        {"role": "assistant", "content": f"Sure. {q2['text']}"},
        {"role": "user", "content": LONG_ANSWER},
    ]
    client.post(
        f"/api/interviews/{data['id']}/transcript",
        json={
            "transcript": transcript,
            "skipped_question_ids": [q1["id"]],
            "ended_early": False,
        },
    )
    report = client.post(f"/api/interviews/{data['id']}/feedback").json()
    assert [a["question_id"] for a in report["answers"]] == [q2["id"]]


def test_history_and_progress(client):
    data = _create_interview(client)
    q1, q2 = data["questions"]
    transcript = [
        {"role": "assistant", "content": q1["text"]},
        {"role": "user", "content": LONG_ANSWER},
    ]
    client.post(
        f"/api/interviews/{data['id']}/transcript",
        json={"transcript": transcript, "skipped_question_ids": [], "ended_early": True},
    )
    client.post(f"/api/interviews/{data['id']}/feedback")

    sessions = client.get("/api/interviews").json()
    assert len(sessions) == 1
    assert sessions[0]["average_score"] is not None

    progress = client.get("/api/progress").json()
    assert len(progress["sessions"]) == 1


def test_mock_chat_flow(client):
    data = _create_interview(client)
    first = client.post(
        f"/api/interviews/{data['id']}/mock-chat", json={"messages": []}
    ).json()
    assert data["questions"][0]["text"] in first["reply"]
    assert first["done"] is False


def test_repeated_interviews_vary_questions(client):
    sets = set()
    for _ in range(8):
        data = _create_interview(client, competencies=[], num_questions=3)
        sets.add(tuple(q["id"] for q in data["questions"]))
    assert len(sets) > 1
