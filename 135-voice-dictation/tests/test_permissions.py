"""Steps 11 and 12 — the blocking setup flow and revocation detection."""

from ccvoice.permissions import (
    REQUIRED,
    CachingChecker,
    FakePermissionChecker,
    PermissionMonitor,
    SetupFlow,
    pick_checker,
)
from ccvoice.ports import Permission


class Prompter:
    """Stands in for `input()`, and can grant a permission between prompts."""

    def __init__(self, answers, on_prompt=None):
        self.answers = list(answers)
        self.on_prompt = on_prompt
        self.count = 0

    def __call__(self, _text=""):
        self.count += 1
        if self.on_prompt:
            self.on_prompt(self.count)
        return self.answers.pop(0) if self.answers else ""


def test_setup_passes_straight_through_when_everything_is_granted():
    checker = FakePermissionChecker()
    lines = []
    assert SetupFlow(checker, Prompter([]), lines.append).run() is True
    assert checker.checks == list(REQUIRED)
    assert "ready to dictate" in "\n".join(lines)


def test_setup_blocks_until_the_permission_is_actually_granted():
    checker = FakePermissionChecker({Permission.MICROPHONE: False, Permission.INPUT_INJECTION: True})
    prompter = Prompter(["", "", ""], on_prompt=lambda n: checker.grant(Permission.MICROPHONE) if n == 3 else None)

    ok = SetupFlow(checker, prompter, lambda _msg: None).run()

    assert ok is True
    assert prompter.count == 3  # it did not move on after the first two attempts
    assert checker.requests == [Permission.MICROPHONE]  # and it nudged the OS once


def test_setup_shows_the_platform_remedy():
    checker = FakePermissionChecker({Permission.MICROPHONE: False})
    lines = []
    SetupFlow(checker, Prompter(["q"]), lines.append).run()
    assert any("fix:" in line for line in lines)


def test_the_user_can_quit_and_setup_reports_failure():
    checker = FakePermissionChecker({Permission.MICROPHONE: False, Permission.INPUT_INJECTION: True})
    lines = []
    assert SetupFlow(checker, Prompter(["q"]), lines.append).run() is False
    assert "Setup stopped" in "\n".join(lines)


def test_setup_gives_up_rather_than_looping_forever():
    checker = FakePermissionChecker({Permission.MICROPHONE: False})
    prompter = Prompter([""] * 100)
    assert SetupFlow(checker, prompter, lambda _m: None, max_rounds=4).run() is False
    assert prompter.count == 4


def test_the_second_permission_is_only_reached_after_the_first():
    checker = FakePermissionChecker({Permission.MICROPHONE: False, Permission.INPUT_INJECTION: False})
    SetupFlow(checker, Prompter(["q"]), lambda _m: None).run()
    assert Permission.INPUT_INJECTION not in checker.checks


def test_the_monitor_notices_a_revoked_permission():
    checker = FakePermissionChecker()
    revoked = []
    monitor = PermissionMonitor(checker, revoked.append)

    monitor.poll()  # baseline: everything granted
    checker.revoke(Permission.INPUT_INJECTION)
    flipped = monitor.poll()

    assert [s.permission for s in flipped] == [Permission.INPUT_INJECTION]
    assert [s.permission for s in revoked] == [Permission.INPUT_INJECTION]


def test_the_monitor_does_not_fire_twice_for_the_same_revocation():
    checker = FakePermissionChecker()
    revoked = []
    monitor = PermissionMonitor(checker, revoked.append)
    monitor.poll()
    checker.revoke(Permission.MICROPHONE)
    monitor.poll()
    monitor.poll()
    assert len(revoked) == 1


def test_the_monitor_reports_a_restored_permission():
    checker = FakePermissionChecker()
    restored = []
    monitor = PermissionMonitor(checker, lambda _s: None, on_restored=restored.append)
    monitor.poll()
    checker.revoke(Permission.MICROPHONE)
    monitor.poll()
    checker.grant(Permission.MICROPHONE)
    monitor.poll()
    assert [s.permission for s in restored] == [Permission.MICROPHONE]


def test_revocation_suspends_dictation_and_restoration_resumes_it(harness):
    checker = FakePermissionChecker()
    h = harness()
    monitor = PermissionMonitor(
        checker, h.app.on_permission_revoked, on_restored=h.app.on_permission_restored
    )
    monitor.poll()

    checker.revoke(Permission.MICROPHONE)
    monitor.poll()
    h.app.on_press()

    assert h.app.blocked_reason.startswith("Microphone access was revoked")
    assert not h.recorder.is_recording
    assert any("Cannot record" in note for note in h.indicator.notifications)

    checker.grant(Permission.MICROPHONE)
    monitor.poll()
    outcome = h.speak(seconds=1.0)

    assert h.app.blocked_reason == ""
    assert outcome.text == "Hello world."


def test_the_caching_checker_reuses_a_recent_answer():
    clock = [0.0]
    inner = FakePermissionChecker()
    checker = CachingChecker(inner, ttl_s=5.0, clock=lambda: clock[0])

    checker.check(Permission.MICROPHONE)
    checker.check(Permission.MICROPHONE)
    assert len(inner.checks) == 1

    clock[0] = 6.0
    checker.check(Permission.MICROPHONE)
    assert len(inner.checks) == 2


def test_the_monitor_invalidates_the_cache_before_polling():
    inner = FakePermissionChecker()
    checker = CachingChecker(inner, ttl_s=10_000)
    monitor = PermissionMonitor(checker, lambda _s: None)
    monitor.poll()
    inner.revoke(Permission.MICROPHONE)
    assert monitor.poll()  # a stale cache would have hidden this


def test_the_real_checker_answers_both_permissions_on_this_platform():
    checker = pick_checker()
    for permission in REQUIRED:
        state = checker.check(permission)
        assert state.permission is permission
        assert isinstance(state.granted, bool)
        assert state.detail or state.remedy
