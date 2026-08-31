"""Tiny calculator — the test fixture. Contains two deliberate bugs."""


def add(a, b):
    return a + b


def subtract(a, b):
    return a + b  # BUG: should be a - b


def multiply(a, b):
    return a * b


def divide(a, b):
    return a / b  # BUG: no zero-division handling (spec: return None)
