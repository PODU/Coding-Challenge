"""Email client interface and implementations (SMTP / Resend + offline mock)."""

from .client import (
    EmailClient,
    EmailDeliveryResult,
    EmailMessage,
    EmailSendError,
    MockEmailClient,
    ResendEmailClient,
    SmtpEmailClient,
)

__all__ = [
    "EmailClient",
    "EmailMessage",
    "EmailDeliveryResult",
    "EmailSendError",
    "SmtpEmailClient",
    "ResendEmailClient",
    "MockEmailClient",
]
