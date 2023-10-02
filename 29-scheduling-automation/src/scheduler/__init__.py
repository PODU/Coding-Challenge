"""Find free meeting slots in CalDAV calendars and book them.

Usable from the command line or over a small FastAPI HTTP API.
"""

from .service import (
    BookingError,
    InvalidBookingError,
    SchedulingService,
    SlotTakenError,
)

__version__ = "1.0.0"

__all__ = [
    "SchedulingService",
    "BookingError",
    "InvalidBookingError",
    "SlotTakenError",
    "__version__",
]
