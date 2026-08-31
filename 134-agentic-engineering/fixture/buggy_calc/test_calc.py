import calc


def test_add():
    assert calc.add(2, 3) == 5


def test_subtract():
    assert calc.subtract(5, 3) == 2


def test_multiply():
    assert calc.multiply(4, 3) == 12


def test_divide():
    assert calc.divide(6, 3) == 2


def test_divide_by_zero_returns_none():
    assert calc.divide(1, 0) is None
