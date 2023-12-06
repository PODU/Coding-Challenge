"""CLI / integration tests over the sample data and error paths."""

from __future__ import annotations

import io

from netmodel.cli import EXIT_INPUT, EXIT_OK, EXIT_USAGE, main


def _run(argv):
    out, err = io.StringIO(), io.StringIO()
    code = main(argv, out=out, err=err)
    return code, out.getvalue(), err.getvalue()


def _links(data_dir):
    return str(data_dir / "links.csv")


def _demands(data_dir):
    return str(data_dir / "demands.csv")


def test_report_over_sample_data(data_dir):
    code, out, _ = _run(
        ["report", "--links", _links(data_dir), "--demands", _demands(data_dir)]
    )
    assert code == EXIT_OK
    assert "# Link Utilization" in out
    assert "LinkId,Start,End,Capacity,Load,UtilizationPct,OverCapacity" in out
    # L2 (B-D) carries 110 of capacity 100 -> 110.00%, over capacity.
    assert "L2,B,D,100,110,110.00,true" in out
    # L6 (D-E) carries 60 of capacity 50 -> 120.00%, over capacity.
    assert "L6,D,E,50,60,120.00,true" in out
    # Route section present and a known route rendered.
    assert "# Routes" in out
    assert "A,D,60,A->B->D,yes" in out


def test_report_writes_out_file(tmp_path, data_dir):
    out_path = tmp_path / "report.csv"
    code, _, _ = _run(
        [
            "report",
            "--links",
            _links(data_dir),
            "--demands",
            _demands(data_dir),
            "--out",
            str(out_path),
        ]
    )
    assert code == EXIT_OK
    content = out_path.read_text(encoding="utf-8")
    assert content.startswith("LinkId,Start,End,Capacity,Load,UtilizationPct,OverCapacity\n")
    assert "L2,B,D,100,110,110.00,true" in content


def test_report_is_deterministic(data_dir):
    args = ["report", "--links", _links(data_dir), "--demands", _demands(data_dir)]
    _, out1, _ = _run(args)
    _, out2, _ = _run(args)
    assert out1 == out2


def test_path_command(data_dir):
    code, out, _ = _run(["path", "--links", _links(data_dir), "--src", "A", "--dst", "D"])
    assert code == EXIT_OK
    assert out.strip() == "A->B->D weight=2"


def test_path_command_no_route(data_dir):
    code, out, _ = _run(["path", "--links", _links(data_dir), "--src", "A", "--dst", "Z"])
    assert code == EXIT_OK
    assert "No path" in out


def test_failure_command_over_sample_data(data_dir):
    code, out, err = _run(
        ["failure", "--links", _links(data_dir), "--demands", _demands(data_dir)]
    )
    assert code == EXIT_OK
    assert out.startswith(
        "FailedLinkId,UnroutableDemands,OverCapacityLinks,MaxUtilizationPct\n"
    )
    # Worst case summary goes to stderr.
    assert "Worst-case single-link failure:" in err
    # The worst scenario is the first data row of the CSV.
    first_data_row = out.splitlines()[1]
    worst_link = first_data_row.split(",")[0]
    assert worst_link in err


def test_failure_is_deterministic(data_dir):
    args = ["failure", "--links", _links(data_dir), "--demands", _demands(data_dir)]
    _, out1, _ = _run(args)
    _, out2, _ = _run(args)
    assert out1 == out2


def test_bad_usage_exit_code(data_dir):
    # Missing required --demands.
    code, _, err = _run(["report", "--links", _links(data_dir)])
    assert code == EXIT_USAGE
    assert "usage error" in err.lower()


def test_no_subcommand_exit_code():
    code, _, _ = _run([])
    assert code == EXIT_USAGE


def test_malformed_input_exit_code(tmp_path):
    bad = tmp_path / "links.csv"
    bad.write_text("LinkId,Start,End,Capacity,Weight\nL1,A,A,100,1\n", encoding="utf-8")
    demands = tmp_path / "demands.csv"
    demands.write_text("Source,Destination,Demand\nA,B,1\n", encoding="utf-8")
    code, _, err = _run(
        ["report", "--links", str(bad), "--demands", str(demands)]
    )
    assert code == EXIT_INPUT
    assert "row 2" in err
