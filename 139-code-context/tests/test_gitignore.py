from ccctx.gitignore import IgnoreMatcher, parse_gitignore, walk_repo


def matcher(text, base=""):
    m = IgnoreMatcher()
    m.rules.extend(parse_gitignore(text, base))
    return m


def test_basename_glob_matches_at_any_depth():
    m = matcher("*.log\n")
    assert m.ignored("debug.log", False)
    assert m.ignored("a/b/debug.log", False)
    assert not m.ignored("debug.txt", False)


def test_negation_reincludes():
    m = matcher("*.log\n!keep.log\n")
    assert m.ignored("x.log", False)
    assert not m.ignored("keep.log", False)


def test_dir_only_pattern_ignores_dirs_not_files():
    m = matcher("build/\n")
    assert m.ignored("build", True)
    assert m.ignored("src/build", True)
    assert not m.ignored("build", False)


def test_anchored_pattern_only_matches_from_base():
    m = matcher("/dist\ndocs/*.html\n")
    assert m.ignored("dist", True)
    assert not m.ignored("src/dist", True)
    assert m.ignored("docs/index.html", False)
    assert not m.ignored("docs/api/index.html", False)


def test_double_star():
    m = matcher("**/cache/**\na/**/z.txt\n")
    assert m.ignored("x/cache/file", False)
    assert m.ignored("a/z.txt", False)
    assert m.ignored("a/b/c/z.txt", False)


def test_comments_and_blank_lines_and_character_class():
    m = matcher("# comment\n\nfile[0-9].txt\n")
    assert m.ignored("file3.txt", False)
    assert not m.ignored("filex.txt", False)
    assert not m.ignored("# comment", False)


def test_nested_gitignore_is_relative_to_its_directory():
    m = matcher("*.tmp\n", base="sub")
    assert m.ignored("sub/a.tmp", False)
    assert m.ignored("sub/deep/a.tmp", False)
    assert not m.ignored("a.tmp", False)


def test_later_rule_wins():
    m = matcher("!important.log\n*.log\n")
    assert m.ignored("important.log", False)


def test_walk_counts_ignored_binary_and_large(sample):
    (sample / "big.txt").write_text("x" * 5000)
    (sample / "sub").mkdir()
    (sample / "sub" / ".gitignore").write_text("secret.txt\n")
    (sample / "sub" / "secret.txt").write_text("s")
    (sample / "sub" / "ok.txt").write_text("ok")
    stats, contents = walk_repo(sample, max_bytes=4000)
    assert "debug.log" not in contents
    assert "keep.log" in contents
    assert "build/out.txt" not in contents
    assert "sub/secret.txt" not in contents and "sub/ok.txt" in contents
    assert not any(p.startswith(".git/") for p in contents)
    assert stats.binary == 1        # logo.png
    assert stats.too_large == 1     # big.txt
    assert stats.ignored == 3       # build/, debug.log, sub/secret.txt
    assert stats.scanned == len(contents) + stats.binary + stats.too_large
