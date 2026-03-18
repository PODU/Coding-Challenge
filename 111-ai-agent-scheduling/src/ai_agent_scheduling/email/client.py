"""``EmailClient`` protocol with SMTP, Resend and mock implementations."""

from __future__ import annotations

from dataclasses import dataclass
from typing import List, Optional, Protocol, runtime_checkable


@dataclass(frozen=True)
class EmailMessage:

    to: str
    subject: str
    body: str


@dataclass(frozen=True)
class EmailDeliveryResult:

    delivered: bool
    provider_id: Optional[str] = None
    error: Optional[str] = None


class EmailSendError(Exception):
    """A single send attempt failed; the runner may retry."""


@runtime_checkable
class EmailClient(Protocol):

    def send(self, message: EmailMessage) -> EmailDeliveryResult:
        ...


class SmtpEmailClient:

    def __init__(
        self,
        host: str,
        port: int = 587,
        *,
        username: Optional[str] = None,
        password: Optional[str] = None,
        sender: Optional[str] = None,
        use_tls: bool = True,
        timeout_s: float = 30.0,
    ):
        self._host = host
        self._port = port
        self._username = username
        self._password = password
        self._sender = sender or username or "ai-agents@localhost"
        self._use_tls = use_tls
        self._timeout_s = timeout_s

    def send(self, message: EmailMessage) -> EmailDeliveryResult:  # pragma: no cover
        import smtplib
        from email.message import EmailMessage as MimeMessage

        mime = MimeMessage()
        mime["From"] = self._sender
        mime["To"] = message.to
        mime["Subject"] = message.subject
        mime.set_content(message.body)
        try:
            with smtplib.SMTP(self._host, self._port, timeout=self._timeout_s) as smtp:
                if self._use_tls:
                    smtp.starttls()
                if self._username and self._password:
                    smtp.login(self._username, self._password)
                smtp.send_message(mime)
        except Exception as exc:
            raise EmailSendError(str(exc)) from exc
        return EmailDeliveryResult(delivered=True)


class ResendEmailClient:

    def __init__(self, api_key: str, sender: str, *, timeout_s: float = 30.0):
        self._api_key = api_key
        self._sender = sender
        self._timeout_s = timeout_s

    def send(self, message: EmailMessage) -> EmailDeliveryResult:  # pragma: no cover
        import json
        import urllib.request

        payload = json.dumps(
            {
                "from": self._sender,
                "to": [message.to],
                "subject": message.subject,
                "text": message.body,
            }
        ).encode("utf-8")
        req = urllib.request.Request(
            "https://api.resend.com/emails",
            data=payload,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout_s) as resp:
                body = json.loads(resp.read().decode("utf-8"))
        except Exception as exc:
            raise EmailSendError(str(exc)) from exc
        return EmailDeliveryResult(delivered=True, provider_id=body.get("id"))


class MockEmailClient:
    """Records sends in ``sent``. The first ``fail_times`` sends raise
    ``EmailSendError``, or every send does with ``always_fail``."""

    def __init__(self, *, fail_times: int = 0, always_fail: bool = False):
        self._remaining_failures = fail_times
        self._always_fail = always_fail
        self.sent: List[EmailMessage] = []
        self.attempts: int = 0

    def send(self, message: EmailMessage) -> EmailDeliveryResult:
        self.attempts += 1
        if self._always_fail:
            raise EmailSendError("scripted permanent failure")
        if self._remaining_failures > 0:
            self._remaining_failures -= 1
            raise EmailSendError("scripted transient failure")
        self.sent.append(message)
        return EmailDeliveryResult(delivered=True, provider_id=f"mock-{len(self.sent)}")
