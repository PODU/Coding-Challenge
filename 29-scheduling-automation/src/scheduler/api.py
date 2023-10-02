"""HTTP API.

* ``GET  /healthz``
* ``GET  /availability``: the configured weekly availability
* ``GET  /slots``: free slots for a duration and window
* ``POST /book``: book a slot and invite an attendee

The service is supplied through a dependency override, which is how tests
plug in a service backed by :class:`FakeProvider`.
"""

from __future__ import annotations

from datetime import datetime, timedelta

from fastapi import Depends, FastAPI, HTTPException, Query
from pydantic import BaseModel, EmailStr, Field

from .domain.interval import Interval
from .domain.slots import SlotRequest
from .service import (
    InvalidBookingError,
    SchedulingService,
    SlotTakenError,
)


class SlotDTO(BaseModel):
    start: datetime
    end: datetime


class AvailabilityRuleDTO(BaseModel):
    weekday: int
    start: str
    end: str


class AvailabilityDTO(BaseModel):
    tz: str
    rules: list[AvailabilityRuleDTO]


class BookRequest(BaseModel):
    start: datetime
    end: datetime
    attendee_name: str = Field(default="")
    attendee_email: EmailStr
    title: str | None = None


class BookResponse(BaseModel):
    uid: str


def get_service() -> SchedulingService:  # pragma: no cover - overridden in app/tests
    """Placeholder that create_app overrides with the real service."""
    raise HTTPException(status_code=503, detail="Service not configured")


def create_app(service: SchedulingService | None = None) -> FastAPI:
    app = FastAPI(title="Scheduling Automation", version="1.0.0")

    if service is not None:
        app.dependency_overrides[get_service] = lambda: service

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/availability", response_model=AvailabilityDTO)
    def availability(svc: SchedulingService = Depends(get_service)) -> AvailabilityDTO:
        return AvailabilityDTO(
            tz=svc.availability.tz,
            rules=[
                AvailabilityRuleDTO(
                    weekday=r.weekday,
                    start=r.start.isoformat(),
                    end=r.end.isoformat(),
                )
                for r in svc.availability.rules
            ],
        )

    @app.get("/slots", response_model=list[SlotDTO])
    def slots(
        duration: int = Query(..., description="Meeting duration in minutes", gt=0),
        from_: datetime = Query(..., alias="from"),
        to: datetime = Query(...),
        step: int = Query(15, description="Slot granularity in minutes", gt=0),
        svc: SchedulingService = Depends(get_service),
    ) -> list[SlotDTO]:
        try:
            req = SlotRequest(
                duration=timedelta(minutes=duration),
                window=Interval(from_, to),
                step=timedelta(minutes=step),
            )
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        return [SlotDTO(start=s.start, end=s.end) for s in svc.get_free_slots(req)]

    @app.post("/book", response_model=BookResponse, status_code=201)
    def book(
        payload: BookRequest,
        svc: SchedulingService = Depends(get_service),
    ) -> BookResponse:
        try:
            slot = Interval(payload.start, payload.end)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        try:
            uid = svc.book(
                slot,
                payload.attendee_name,
                str(payload.attendee_email),
                payload.title,
            )
        except InvalidBookingError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        except SlotTakenError as exc:
            raise HTTPException(status_code=409, detail=str(exc)) from exc
        return BookResponse(uid=uid)

    return app
