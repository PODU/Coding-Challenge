"""FastAPI interface smoke tests using a fake-provider-backed service."""

from datetime import UTC, datetime, time

import pytest
from fastapi.testclient import TestClient

from scheduler.api import create_app
from scheduler.domain.availability import AvailabilityRule, WeeklyAvailability
from scheduler.domain.clock import FixedClock
from scheduler.domain.interval import Interval
from scheduler.providers.fake_provider import FakeProvider
from scheduler.providers.ics import parse_events
from scheduler.service import SchedulingService

from .conftest import read_fixture


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


@pytest.fixture
def client() -> TestClient:
    window = Interval(utc(2026, 7, 2), utc(2026, 7, 3))
    events = parse_events(read_fixture("work.ics"), "work", window)
    events += parse_events(read_fixture("personal.ics"), "personal", window)
    provider = FakeProvider(events)
    availability = WeeklyAvailability(
        tz="UTC", rules=tuple(AvailabilityRule(d, time(9), time(17)) for d in range(5))
    )
    service = SchedulingService(
        provider=provider,
        availability=availability,
        clock=FixedClock(utc(2026, 7, 1)),
        organizer_email="alice@example.com",
    )
    return TestClient(create_app(service))


def test_healthz(client):
    assert client.get("/healthz").json() == {"status": "ok"}


def test_availability(client):
    body = client.get("/availability").json()
    assert body["tz"] == "UTC"
    assert len(body["rules"]) == 5


def test_slots_endpoint(client):
    resp = client.get(
        "/slots",
        params={
            "duration": 60,
            "from": "2026-07-02T00:00:00Z",
            "to": "2026-07-03T00:00:00Z",
            "step": 60,
        },
    )
    assert resp.status_code == 200
    starts = [datetime.fromisoformat(s["start"]).hour for s in resp.json()]
    assert starts == [10, 11, 12, 13, 16]


def test_book_then_conflict(client):
    payload = {
        "start": "2026-07-02T11:00:00Z",
        "end": "2026-07-02T12:00:00Z",
        "attendee_name": "Sam",
        "attendee_email": "sam@example.com",
        "title": "Intro call",
    }
    first = client.post("/book", json=payload)
    assert first.status_code == 201
    assert first.json()["uid"]

    second = client.post("/book", json=payload)
    assert second.status_code == 409


def test_book_invalid_email(client):
    payload = {
        "start": "2026-07-02T11:00:00Z",
        "end": "2026-07-02T12:00:00Z",
        "attendee_email": "nope",
    }
    resp = client.post("/book", json=payload)
    assert resp.status_code == 422  # pydantic EmailStr validation
