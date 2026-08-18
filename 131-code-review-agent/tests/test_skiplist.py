from reviewagent.github import ChangedFile
from reviewagent.skiplist import partition, skip_reason


def cf(name, patch="@@ -1 +1 @@\n+x"):
    return ChangedFile(filename=name, status="modified",
                       additions=1, deletions=0, patch=patch)


def test_lock_files_skipped():
    for name in ["uv.lock", "package-lock.json", "yarn.lock", "Cargo.lock",
                 "poetry.lock", "sub/dir/Gemfile.lock"]:
        assert skip_reason(name) is not None, name


def test_vendored_paths_skipped():
    assert skip_reason("vendor/lib/foo.py")
    assert skip_reason("frontend/node_modules/react/index.js")
    assert skip_reason("dist/bundle.js")


def test_minified_and_binary_skipped():
    assert skip_reason("static/app.min.js")
    assert skip_reason("logo.svg")
    assert skip_reason("proto/service_pb2.py")


def test_normal_source_not_skipped():
    assert skip_reason("app/config.py") is None
    assert skip_reason("src/locker.py") is None  # 'lock' in name != lock file
    assert skip_reason("tests/test_orders.py") is None


def test_partition_separates_and_gives_reasons():
    files = [cf("app/main.py"), cf("uv.lock"), cf("data.bin", patch="")]
    to_review, skipped = partition(files)
    assert [f.filename for f in to_review] == ["app/main.py"]
    reasons = dict((f.filename, r) for f, r in skipped)
    assert "uv.lock" in reasons
    assert "binary" in reasons["data.bin"]


def test_missing_patch_skipped_even_for_source():
    to_review, skipped = partition([cf("app/huge.py", patch="")])
    assert not to_review
    assert skipped[0][0].filename == "app/huge.py"
