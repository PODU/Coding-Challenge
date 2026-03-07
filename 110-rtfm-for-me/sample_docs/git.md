# git

## NAME

git - the stupid content tracker

## SYNOPSIS

git [--version] [--help] [-C <path>] <command> [<args>]

## DESCRIPTION

Git is a fast, scalable, distributed revision control system with an unusually
rich command set that provides both high-level operations and full access to
internals.

## COMMIT

The `git commit` command records changes to the repository. It stores the
current contents of the index in a new commit along with a log message from the
user describing the changes.

Use `git commit -m "message"` to record a commit with the given message without
opening an editor. Use `git commit -a` to automatically stage files that have
been modified and deleted, but new files you have not told Git about are not
affected.

The `--amend` flag replaces the tip of the current branch by creating a new
commit. This is useful for fixing the most recent commit message or adding
forgotten changes.

## LOG

The `git log` command shows the commit history. By default it lists each commit
with its author, date, and message. The `--oneline` flag condenses each commit
to a single line. The `-n <number>` flag limits the number of commits shown.

The `--graph` flag draws a text-based graphical representation of the commit
history on the left side of the output.

## RESET

The `git reset` command resets the current HEAD to a specified state. The
`--soft` option keeps your changes staged, `--mixed` (the default) unstages
them but keeps the working tree, and `--hard` discards all changes in the
working tree. Use `--hard` with caution because it is destructive.

## SEE ALSO

git-commit(1), git-log(1), git-reset(1)
