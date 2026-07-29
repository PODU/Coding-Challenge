"""FastAPI app for the mock interview coach."""

from __future__ import annotations

import random
from pathlib import Path

from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, Field

from . import feedback as feedback_mod
from . import mock_interviewer, personas, questions
from .anam import AnamClient, AnamError, HttpAnamClient, MockAnamClient
from .config import Settings, get_settings
from .llm import AnthropicFeedbackClient, FeedbackClient, FeedbackError, MockFeedbackClient
from .storage import Storage

STATIC_DIR = Path(__file__).resolve().parents[2] / "static"


class CreateInterviewRequest(BaseModel):
    role: str = Field(default="Software Engineer", max_length=100)
    seniority: str = Field(default="mid-level", max_length=50)
    competencies: list[str] = Field(default_factory=list)
    style: str = personas.DEFAULT_STYLE
    num_questions: int = Field(default=3, ge=1, le=8)


class TranscriptTurn(BaseModel):
    role: str
    content: str


class SaveTranscriptRequest(BaseModel):
    transcript: list[TranscriptTurn]
    skipped_question_ids: list[str] = Field(default_factory=list)
    ended_early: bool = False


class MockChatRequest(BaseModel):
    messages: list[TranscriptTurn]


def create_app(
    settings: Settings | None = None,
    anam_client: AnamClient | None = None,
    feedback_client: FeedbackClient | None = None,
) -> FastAPI:
    settings = settings or get_settings()

    if anam_client is None:
        anam_client = (
            HttpAnamClient(settings.anam_api_key)
            if settings.anam_configured
            else MockAnamClient()
        )
    if feedback_client is None:
        feedback_client = (
            AnthropicFeedbackClient(settings.anthropic_api_key, settings.feedback_model)
            if settings.anthropic_configured
            else MockFeedbackClient()
        )

    storage = Storage(settings.data_dir)
    app = FastAPI(title="Mock Interview Coach", version="1.0.0")
    mock_video = isinstance(anam_client, MockAnamClient)

    def load_record(interview_id: str) -> dict:
        try:
            return storage.get(interview_id)
        except KeyError:
            raise HTTPException(404, "Unknown interview id")

    @app.get("/api/config")
    def get_config():
        return {
            "mock_video": mock_video,
            "mock_feedback": isinstance(feedback_client, MockFeedbackClient),
            "max_session_seconds": settings.max_session_seconds,
            "competencies": questions.COMPETENCIES,
            "styles": personas.list_styles(),
        }

    @app.post("/api/interviews")
    def create_interview(req: CreateInterviewRequest):
        picked = questions.select_questions(
            req.competencies, req.num_questions, random.Random()
        )
        q_dicts = [{"id": q.id, "competency": q.competency, "text": q.text} for q in picked]
        system_prompt = personas.build_system_prompt(
            req.role, req.seniority, picked, req.style
        )
        record = storage.create(
            setup={
                "role": req.role,
                "seniority": req.seniority,
                "competencies": req.competencies or questions.COMPETENCIES,
                "style": req.style,
            },
            questions=q_dicts,
            system_prompt=system_prompt,
        )
        return {"id": record["id"], "questions": q_dicts, "setup": record["setup"]}

    @app.post("/api/session-token")
    def session_token(body: dict):
        record = load_record(body.get("interview_id", ""))
        picked = [
            questions.Question(q["id"], q["competency"], q["text"])
            for q in record["questions"]
        ]
        persona_config = personas.build_persona_config(
            record["setup"]["role"],
            record["setup"]["seniority"],
            picked,
            record["setup"]["style"],
        )
        try:
            token = anam_client.create_session_token(persona_config)
        except AnamError as exc:
            raise HTTPException(502, str(exc))
        return {
            "session_token": token,
            "mock": mock_video,
            "max_session_seconds": settings.max_session_seconds,
        }

    @app.get("/api/interviews")
    def list_interviews():
        return storage.summaries()

    @app.get("/api/interviews/{interview_id}")
    def get_interview(interview_id: str):
        record = load_record(interview_id)
        record.pop("system_prompt", None)
        return record

    @app.post("/api/interviews/{interview_id}/transcript")
    def save_transcript(interview_id: str, req: SaveTranscriptRequest):
        record = load_record(interview_id)
        record["transcript"] = [t.model_dump() for t in req.transcript]
        record["skipped_question_ids"] = req.skipped_question_ids
        record["ended_early"] = req.ended_early
        storage.save(record)
        return {"ok": True, "turns": len(record["transcript"])}

    @app.post("/api/interviews/{interview_id}/feedback")
    def make_feedback(interview_id: str):
        record = load_record(interview_id)
        if not record["transcript"]:
            raise HTTPException(400, "No transcript saved for this interview yet.")
        try:
            report = feedback_mod.generate_report(
                feedback_client,
                record["questions"],
                record["transcript"],
                record["setup"]["role"],
                record["setup"]["seniority"],
                record["skipped_question_ids"],
            )
        except FeedbackError as exc:
            raise HTTPException(502, str(exc))
        record["feedback"] = report
        storage.save(record)
        return report

    @app.get("/api/progress")
    def progress():
        points = [
            {"created_at": s["created_at"], "average_score": s["average_score"], "id": s["id"]}
            for s in reversed(storage.summaries())
            if s["average_score"] is not None
        ]
        return {"sessions": points}

    @app.post("/api/interviews/{interview_id}/mock-chat")
    def mock_chat(interview_id: str, req: MockChatRequest):
        record = load_record(interview_id)
        q_texts = [q["text"] for q in record["questions"]]
        return mock_interviewer.next_reply(
            q_texts, [m.model_dump() for m in req.messages]
        )

    if STATIC_DIR.exists():
        @app.get("/")
        def index():
            return FileResponse(STATIC_DIR / "index.html")

        app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")

    return app
