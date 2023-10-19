"""Reed-Solomon generator-polynomial and EC-codeword known-vector tests."""
from qrgen import galois, reed_solomon
from qrgen.analyze import make_plan
from qrgen.encode import encode_data_codewords


def test_generator_poly_degree():
    for deg in (7, 10, 13, 15, 30):
        g = reed_solomon.generator_poly(deg)
        assert len(g) == deg + 1
        assert g[0] == 1  # monic


def test_generator_poly_known_coeffs():
    # Degree-7 generator (Thonky): exponents [0,87,229,146,149,238,102,21]
    # of alpha; convert to integer coefficients via the antilog table.
    exps = [0, 87, 229, 146, 149, 238, 102, 21]
    expected = [galois.EXP[e] for e in exps]
    assert list(reed_solomon.generator_poly(7)) == expected


def test_generator_poly_deg10_known():
    # Degree-10 generator exponents (Thonky).
    exps = [0, 251, 67, 46, 61, 118, 70, 64, 94, 32, 45]
    expected = [galois.EXP[e] for e in exps]
    assert list(reed_solomon.generator_poly(10)) == expected


def test_ec_codewords_hello_world_1q():
    # Classic Thonky worked example: "HELLO WORLD", version 1, EC level Q.
    plan = make_plan("HELLO WORLD", ec="Q")
    assert plan.version == 1 and plan.mode == "alphanumeric"
    data = encode_data_codewords(plan)
    assert data == [32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236]
    ec = reed_solomon.ec_codewords(data, 13)
    assert ec == [168, 72, 22, 82, 217, 54, 156, 0, 46, 15, 180, 122, 16]


def test_ec_codewords_form_valid_codeword():
    # A correct RS codeword evaluates to zero at alpha^0..alpha^(n_ec-1).
    plan = make_plan("HELLO WORLD", ec="Q")
    data = encode_data_codewords(plan)
    ec = reed_solomon.ec_codewords(data, 13)
    full = data + ec
    for i in range(13):
        syndrome = 0
        x = galois.EXP[i]
        for coef in full:
            syndrome = galois.mul(syndrome, x) ^ coef
        assert syndrome == 0
