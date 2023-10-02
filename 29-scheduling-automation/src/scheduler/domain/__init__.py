"""Pure, deterministic scheduling domain (no I/O, no wall-clock reads)."""

from .availability import AvailabilityRule, WeeklyAvailability
from .clock import Clock, FixedClock, SystemClock
from .interval import Interval, intersect, merge, subtract
from .slots import SlotRequest, free_slots

__all__ = [
    "AvailabilityRule",
    "WeeklyAvailability",
    "Clock",
    "FixedClock",
    "SystemClock",
    "Interval",
    "intersect",
    "merge",
    "subtract",
    "SlotRequest",
    "free_slots",
]
