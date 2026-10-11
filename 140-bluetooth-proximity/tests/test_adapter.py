import asyncio

import bleak
import pytest
from bleak.exc import BleakBluetoothNotAvailableError, BleakBluetoothNotAvailableReason as R
from bleak.exc import BleakError

from ccprox import scanner
from ccprox.scanner import EXIT_ADAPTER, EXIT_PERMISSION, BleakSource, translate


@pytest.mark.parametrize("reason,code,text", [
    (R.NO_BLUETOOTH, EXIT_ADAPTER, "No Bluetooth LE adapter"),
    (R.NO_BLE_CENTRAL_ROLE, EXIT_ADAPTER, "No Bluetooth LE adapter"),
    (R.POWERED_OFF, EXIT_ADAPTER, "powered off"),
    (R.DENIED_BY_USER, EXIT_PERMISSION, "denied"),
    (R.DENIED_BY_SYSTEM, EXIT_PERMISSION, "denied"),
])
def test_translate_bleak_reasons(reason, code, text):
    e = translate(BleakBluetoothNotAvailableError("x", reason))
    assert e.exit_code == code
    assert text in str(e)


@pytest.mark.parametrize("msg,code,text", [
    ("No Bluetooth adapters found.", EXIT_ADAPTER, "No Bluetooth LE adapter"),
    ("[org.bluez.Error.NotReady] Resource Not Ready", EXIT_ADAPTER, "powered off"),
    ("org.freedesktop.DBus.Error.AccessDenied", EXIT_PERMISSION, "denied"),
    ("something odd", EXIT_ADAPTER, "something odd"),
])
def test_translate_messages(msg, code, text):
    e = translate(BleakError(msg))
    assert e.exit_code == code and text in str(e)


def test_messages_are_actionable():
    assert "retry" in str(translate(BleakBluetoothNotAvailableError("x", R.POWERED_OFF)))
    assert "Settings" in str(translate(BleakBluetoothNotAvailableError("x", R.DENIED_BY_USER))) \
        or "group" in str(translate(BleakBluetoothNotAvailableError("x", R.DENIED_BY_USER)))


class FailingScanner:
    exc = None

    def __init__(self, **kw):
        pass

    async def start(self):
        if self.exc is None:
            await asyncio.sleep(3600)  # adapter that never answers
        raise self.exc

    async def stop(self):
        pass


@pytest.mark.parametrize("exc,code,text", [
    (BleakBluetoothNotAvailableError("off", R.POWERED_OFF), 3, "powered off"),
    (BleakBluetoothNotAvailableError("none", R.NO_BLUETOOTH), 3, "No Bluetooth LE adapter"),
    (BleakBluetoothNotAvailableError("no", R.DENIED_BY_USER), 4, "denied"),
    (OSError("The device is not ready"), 3, "powered off"),
])
def test_cli_reports_adapter_errors_cleanly(run_cli, monkeypatch, exc, code, text):
    monkeypatch.setattr(bleak, "BleakScanner", type("S", (FailingScanner,), {"exc": exc}))
    rc, out, err = run_cli(["--duration", "1"],
                           source_factory=lambda q, clock: BleakSource(q, clock=clock))
    assert rc == code
    assert err.startswith("ccprox: ") and text in err
    assert "Traceback" not in err


def test_cli_does_not_hang_on_unresponsive_adapter(run_cli, monkeypatch):
    monkeypatch.setattr(bleak, "BleakScanner", FailingScanner)
    monkeypatch.setattr(scanner, "START_TIMEOUT_S", 0.2)
    rc, _, err = run_cli(["--hunt", "x"], source_factory=lambda q, clock: BleakSource(q, clock=clock))
    assert rc == 3 and "not responding" in err
