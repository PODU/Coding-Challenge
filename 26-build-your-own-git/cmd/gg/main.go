// Command gg is a small, Git-compatible version control client. It reads and
// writes the real on-disk Git format (loose objects, the v2 index, tree/commit
// objects, refs) so repositories it creates interoperate with canonical git.
package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gg/internal/diff"
	"gg/internal/ignore"
	idxpkg "gg/internal/index"
	"gg/internal/object"
	"gg/internal/repo"
	"gg/internal/status"
	"gg/internal/transport"
)

const usage = `gg - a small Git-compatible client

usage:
  gg init [<dir>]
  gg hash-object [-w] [-t <type>] <file>
  gg cat-file (-t | -p | -s) <oid>
  gg add <pathspec>...
  gg status [--porcelain]
  gg commit -m <message> [--author "Name <email>"]
  gg log [-n <count>]
  gg diff [<pathspec>...]
  gg push <remote-url> [<branch>]
`

// Exit codes: 0 success, 1 runtime error, 2 usage error.
const (
	exitOK    = 0
	exitErr   = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	cmd, rest := args[0], args[1:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(rest)
	case "hash-object":
		err = cmdHashObject(rest)
	case "cat-file":
		err = cmdCatFile(rest)
	case "add":
		err = cmdAdd(rest)
	case "status":
		err = cmdStatus(rest)
	case "commit":
		err = cmdCommit(rest)
	case "log":
		err = cmdLog(rest)
	case "diff":
		err = cmdDiff(rest)
	case "push":
		err = cmdPush(rest)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "gg: unknown command %q\n%s", cmd, usage)
		return exitUsage
	}

	if err != nil {
		if ue, ok := err.(usageError); ok {
			fmt.Fprintf(os.Stderr, "gg: %s\n", ue.msg)
			return exitUsage
		}
		fmt.Fprintf(os.Stderr, "gg: %v\n", err)
		return exitErr
	}
	return exitOK
}

// usageError signals an exit-code-2 misuse.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }
func usageErrf(format string, a ...any) error {
	return usageError{fmt.Sprintf(format, a...)}
}

func openRepo() (*repo.Repo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	r, err := repo.Open(wd)
	if err != nil {
		return nil, usageError{err.Error()}
	}
	return r, nil
}

func cmdInit(args []string) error {
	dir := "."
	if len(args) >= 1 {
		dir = args[0]
	}
	r, err := repo.Init(dir)
	if err != nil {
		return err
	}
	fmt.Printf("Initialized empty Git repository in %s\n", r.GitDir)
	return nil
}

func cmdHashObject(args []string) error {
	write := false
	typ := object.TypeBlob
	var file string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-w":
			write = true
		case "-t":
			if i+1 >= len(args) {
				return usageErrf("hash-object: -t requires a type")
			}
			i++
			typ = object.Type(args[i])
		default:
			file = args[i]
		}
	}
	if file == "" {
		return usageErrf("hash-object: missing file")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	if write {
		r, err := openRepo()
		if err != nil {
			return err
		}
		oid, err := object.Write(r.GitDir, typ, data)
		if err != nil {
			return err
		}
		fmt.Println(oid)
		return nil
	}
	fmt.Println(object.Hash(typ, data))
	return nil
}

func cmdCatFile(args []string) error {
	if len(args) != 2 {
		return usageErrf("cat-file: usage: cat-file (-t|-p|-s) <oid>")
	}
	flag, oid := args[0], args[1]
	r, err := openRepo()
	if err != nil {
		return err
	}
	t, payload, err := object.Read(r.GitDir, oid)
	if err != nil {
		return err
	}
	switch flag {
	case "-t":
		fmt.Println(t)
	case "-s":
		fmt.Println(len(payload))
	case "-p":
		return prettyPrint(t, payload)
	default:
		return usageErrf("cat-file: unknown flag %q", flag)
	}
	return nil
}

func prettyPrint(t object.Type, payload []byte) error {
	switch t {
	case object.TypeTree:
		entries, err := object.DecodeTree(payload)
		if err != nil {
			return err
		}
		for _, e := range entries {
			kind := "blob"
			mode := e.Mode
			if e.Mode == object.ModeTree {
				kind = "tree"
			}
			// Git pads tree modes to 6 digits.
			if len(mode) == 5 {
				mode = "0" + mode
			}
			fmt.Printf("%s %s %s\t%s\n", mode, kind, e.OID, e.Name)
		}
	default:
		os.Stdout.Write(payload)
	}
	return nil
}

func cmdAdd(args []string) error {
	if len(args) == 0 {
		return usageErrf("add: nothing specified, nothing added")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	idx, err := idxpkg.Read(r.IndexPath())
	if err != nil {
		return err
	}
	matcher, err := ignore.FromFile(filepath.Join(r.Worktree, ".gitignore"))
	if err != nil {
		return err
	}

	for _, spec := range args {
		if err := addPath(r, idx, matcher, spec); err != nil {
			return err
		}
	}
	return idx.Write(r.IndexPath())
}

func addPath(r *repo.Repo, idx *idxpkg.Index, matcher *ignore.Matcher, spec string) error {
	abs, err := filepath.Abs(spec)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		// Possibly a deleted file still in the index: stage its removal.
		rel, relErr := r.RelPath(spec)
		if relErr == nil {
			if _, ok := idx.Get(rel); ok {
				idx.Remove(rel)
				return nil
			}
		}
		return err
	}

	if info.IsDir() {
		return filepath.Walk(abs, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := r.RelPath(p)
			if relErr != nil {
				return relErr
			}
			if fi.IsDir() {
				if rel == ".git" || strings.HasPrefix(rel, ".git/") {
					return filepath.SkipDir
				}
				if rel != "." && matcher.Match(rel, true) {
					return filepath.SkipDir
				}
				return nil
			}
			if matcher.Match(rel, false) {
				return nil
			}
			return stageFile(r, idx, p)
		})
	}

	rel, err := r.RelPath(abs)
	if err != nil {
		return err
	}
	if matcher.Match(rel, false) {
		return nil
	}
	return stageFile(r, idx, abs)
}

// stageFile writes the file's blob and updates its index entry.
func stageFile(r *repo.Repo, idx *idxpkg.Index, absPath string) error {
	rel, err := r.RelPath(absPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	oid, err := object.Write(r.GitDir, object.TypeBlob, data)
	if err != nil {
		return err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	e := idxpkg.Entry{
		MtimeSec:  uint32(info.ModTime().Unix()),
		MtimeNsec: uint32(info.ModTime().Nanosecond()),
		CtimeSec:  uint32(info.ModTime().Unix()),
		CtimeNsec: uint32(info.ModTime().Nanosecond()),
		Mode:      fileMode(info),
		Size:      uint32(info.Size()),
		Path:      rel,
	}
	if err := e.SetOIDHex(oid); err != nil {
		return err
	}
	idx.Add(e)
	return nil
}

func fileMode(info os.FileInfo) uint32 {
	m := info.Mode()
	if m&os.ModeSymlink != 0 {
		return idxpkg.ModeSymlink
	}
	if m&0o111 != 0 {
		return idxpkg.ModeExec
	}
	return idxpkg.ModeRegular
}

func cmdStatus(args []string) error {
	porcelain := false
	for _, a := range args {
		if a == "--porcelain" {
			porcelain = true
		}
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	s, err := status.Compute(r)
	if err != nil {
		return err
	}

	if porcelain {
		printPorcelain(s)
		return nil
	}
	printStatus(s)
	return nil
}

func printPorcelain(s *status.Status) {
	type row struct{ code, path string }
	var rows []row
	stagedKind := map[string]status.ChangeKind{}
	for _, c := range s.Staged {
		stagedKind[c.Path] = c.Kind
	}
	unstagedKind := map[string]status.ChangeKind{}
	for _, c := range s.Unstaged {
		unstagedKind[c.Path] = c.Kind
	}
	paths := map[string]bool{}
	for p := range stagedKind {
		paths[p] = true
	}
	for p := range unstagedKind {
		paths[p] = true
	}
	for p := range paths {
		x := porcelainCode(stagedKind, p)
		y := porcelainCode(unstagedKind, p)
		rows = append(rows, row{string(x) + string(y), p})
	}
	for _, u := range s.Untracked {
		rows = append(rows, row{"??", u})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	for _, r := range rows {
		fmt.Printf("%s %s\n", r.code, r.path)
	}
}

func porcelainCode(m map[string]status.ChangeKind, p string) byte {
	k, ok := m[p]
	if !ok {
		return ' '
	}
	switch k {
	case status.Added:
		return 'A'
	case status.Modified:
		return 'M'
	case status.Deleted:
		return 'D'
	}
	return ' '
}

func printStatus(s *status.Status) {
	if s.Branch != "" {
		fmt.Printf("On branch %s\n", s.Branch)
	} else {
		fmt.Println("HEAD detached")
	}
	if s.Unborn {
		fmt.Println("\nNo commits yet")
	}

	if len(s.Staged) > 0 {
		fmt.Println("\nChanges to be committed:")
		for _, c := range s.Staged {
			fmt.Printf("\t%s:   %s\n", c.Kind, c.Path)
		}
	}
	if len(s.Unstaged) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		for _, c := range s.Unstaged {
			fmt.Printf("\t%s:   %s\n", c.Kind, c.Path)
		}
	}
	if len(s.Untracked) > 0 {
		fmt.Println("\nUntracked files:")
		for _, u := range s.Untracked {
			fmt.Printf("\t%s\n", u)
		}
	}
	if s.Clean() {
		fmt.Println("\nnothing to commit, working tree clean")
	}
}

func cmdCommit(args []string) error {
	var message string
	var authorOverride string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m":
			if i+1 >= len(args) {
				return usageErrf("commit: -m requires a message")
			}
			i++
			message = args[i]
		case "--author":
			if i+1 >= len(args) {
				return usageErrf("commit: --author requires an argument")
			}
			i++
			authorOverride = args[i]
		default:
			return usageErrf("commit: unexpected argument %q", args[i])
		}
	}
	if message == "" {
		return usageErrf("commit: missing -m <message>")
	}

	r, err := openRepo()
	if err != nil {
		return err
	}
	idx, err := idxpkg.Read(r.IndexPath())
	if err != nil {
		return err
	}
	if len(idx.Entries) == 0 {
		return fmt.Errorf("nothing to commit (index is empty)")
	}

	treeOID, err := writeTree(r, idx.Entries, "")
	if err != nil {
		return err
	}

	parent, err := r.HeadOID()
	if err != nil {
		return err
	}
	var parents []string
	if parent != "" {
		parents = append(parents, parent)
	}

	author, err := resolveSignature(r, authorOverride)
	if err != nil {
		return err
	}
	commit := object.Commit{
		Tree:      treeOID,
		Parents:   parents,
		Author:    author,
		Committer: author,
		Message:   message,
	}
	commitOID, err := object.Write(r.GitDir, object.TypeCommit, object.EncodeCommit(commit))
	if err != nil {
		return err
	}

	ref, _, err := r.HEADRef()
	if err != nil {
		return err
	}
	if ref == "" {
		ref = "refs/heads/" + repo.DefaultBranch
	}
	if err := r.UpdateRef(ref, commitOID); err != nil {
		return err
	}

	short := commitOID
	if len(short) > 7 {
		short = short[:7]
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	rootNote := ""
	if len(parents) == 0 {
		rootNote = " (root-commit)"
	}
	fmt.Printf("[%s%s %s] %s\n", branch, rootNote, short, firstLine(message))
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// writeTree writes the tree for directory prefix ("" for the root), and all
// its subtrees, from the index entries. It returns the tree's OID.
func writeTree(r *repo.Repo, entries []idxpkg.Entry, prefix string) (string, error) {
	var treeEntries []object.TreeEntry
	subdirs := []string{}
	subSeen := map[string]bool{}

	pfx := ""
	if prefix != "" {
		pfx = prefix + "/"
	}

	for _, e := range entries {
		if pfx != "" && !strings.HasPrefix(e.Path, pfx) {
			continue
		}
		rel := strings.TrimPrefix(e.Path, pfx)
		if rel == "" {
			continue
		}
		if idx := strings.IndexByte(rel, '/'); idx >= 0 {
			name := rel[:idx]
			if !subSeen[name] {
				subSeen[name] = true
				subdirs = append(subdirs, name)
			}
			continue
		}
		mode := object.ModeFile
		if e.Mode == idxpkg.ModeExec {
			mode = object.ModeExec
		} else if e.Mode == idxpkg.ModeSymlink {
			mode = object.ModeSymlink
		}
		treeEntries = append(treeEntries, object.TreeEntry{
			Mode: mode,
			Name: rel,
			OID:  e.OIDHex(),
		})
	}

	for _, name := range subdirs {
		childPrefix := name
		if prefix != "" {
			childPrefix = prefix + "/" + name
		}
		oid, err := writeTree(r, entries, childPrefix)
		if err != nil {
			return "", err
		}
		treeEntries = append(treeEntries, object.TreeEntry{
			Mode: object.ModeTree,
			Name: name,
			OID:  oid,
		})
	}

	payload, err := object.EncodeTree(treeEntries)
	if err != nil {
		return "", err
	}
	return object.Write(r.GitDir, object.TypeTree, payload)
}

// resolveSignature picks the author/committer identity.
func resolveSignature(r *repo.Repo, override string) (object.Signature, error) {
	now := time.Now()
	if override != "" {
		name, email, err := parseNameEmail(override)
		if err != nil {
			return object.Signature{}, err
		}
		return object.Signature{Name: name, Email: email, When: now}, nil
	}

	name := os.Getenv("GG_AUTHOR_NAME")
	email := os.Getenv("GG_AUTHOR_EMAIL")
	cfgName, cfgEmail := readUserConfig(r)
	if name == "" {
		name = cfgName
	}
	if email == "" {
		email = cfgEmail
	}
	if name == "" {
		name = "GG User"
	}
	if email == "" {
		email = "gg@example.invalid"
	}
	return object.Signature{Name: name, Email: email, When: now}, nil
}

func parseNameEmail(s string) (name, email string, err error) {
	lt := strings.Index(s, "<")
	gt := strings.Index(s, ">")
	if lt < 0 || gt < 0 || gt < lt {
		return "", "", usageErrf("invalid author %q (want \"Name <email>\")", s)
	}
	return strings.TrimSpace(s[:lt]), s[lt+1 : gt], nil
}

// readUserConfig reads [user] name/email from .git/config (best effort).
func readUserConfig(r *repo.Repo) (name, email string) {
	f, err := os.Open(filepath.Join(r.GitDir, "config"))
	if err != nil {
		return "", ""
	}
	defer f.Close()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			section = strings.Fields(section)[0]
			continue
		}
		if section != "user" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		val := strings.TrimSpace(kv[1])
		switch key {
		case "name":
			name = val
		case "email":
			email = val
		}
	}
	return name, email
}

func cmdLog(args []string) error {
	limit := -1
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" {
			if i+1 >= len(args) {
				return usageErrf("log: -n requires a count")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return usageErrf("log: invalid count %q", args[i])
			}
			limit = n
		}
	}

	r, err := openRepo()
	if err != nil {
		return err
	}
	oid, err := r.HeadOID()
	if err != nil {
		return err
	}
	if oid == "" {
		return fmt.Errorf("no commits yet")
	}

	count := 0
	for oid != "" {
		if limit >= 0 && count >= limit {
			break
		}
		t, payload, err := object.Read(r.GitDir, oid)
		if err != nil {
			return err
		}
		if t != object.TypeCommit {
			return fmt.Errorf("object %s is not a commit", oid)
		}
		c, err := object.DecodeCommit(payload)
		if err != nil {
			return err
		}
		fmt.Printf("commit %s\n", oid)
		fmt.Printf("Author: %s <%s>\n", c.Author.Name, c.Author.Email)
		fmt.Printf("Date:   %s\n", c.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700"))
		fmt.Println()
		for _, line := range strings.Split(strings.TrimRight(c.Message, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
		count++
		if len(c.Parents) == 0 {
			break
		}
		oid = c.Parents[0]
	}
	return nil
}

func cmdDiff(args []string) error {
	r, err := openRepo()
	if err != nil {
		return err
	}
	idx, err := idxpkg.Read(r.IndexPath())
	if err != nil {
		return err
	}

	filter := map[string]bool{}
	for _, a := range args {
		if rel, err := r.RelPath(a); err == nil {
			filter[rel] = true
		}
	}

	for _, e := range idx.Entries {
		if len(filter) > 0 && !filter[e.Path] {
			continue
		}
		_, blob, err := object.Read(r.GitDir, e.OIDHex())
		if err != nil {
			return err
		}
		full := filepath.Join(r.Worktree, filepath.FromSlash(e.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				out := diff.Unified("a/"+e.Path, "/dev/null", string(blob), "")
				if out != "" {
					fmt.Printf("diff --git a/%s b/%s\n", e.Path, e.Path)
					fmt.Print(out)
				}
				continue
			}
			return err
		}
		out := diff.Unified("a/"+e.Path, "b/"+e.Path, string(blob), string(data))
		if out != "" {
			fmt.Printf("diff --git a/%s b/%s\n", e.Path, e.Path)
			fmt.Print(out)
		}
	}
	return nil
}

func cmdPush(args []string) error {
	if len(args) < 1 {
		return usageErrf("push: usage: push <remote-url> [<branch>]")
	}
	remoteURL := args[0]
	r, err := openRepo()
	if err != nil {
		return err
	}

	branch := r.CurrentBranch()
	if len(args) >= 2 {
		branch = args[1]
	}
	if branch == "" {
		return fmt.Errorf("push: cannot determine branch (detached HEAD); specify one")
	}
	ref := "refs/heads/" + branch

	localOID, err := r.ReadRef(ref)
	if err != nil {
		return err
	}
	if localOID == "" {
		return fmt.Errorf("push: %s has no commits", ref)
	}

	cleanURL, auth := splitAuth(remoteURL)

	client := &http.Client{Timeout: 60 * time.Second}
	remoteRefs, err := transport.DiscoverReceivePack(client, cleanURL, auth)
	if err != nil {
		return err
	}

	remoteOID := transport.ZeroOID
	have := map[string]bool{}
	for _, rr := range remoteRefs {
		have[rr.OID] = true
		if rr.Name == ref {
			remoteOID = rr.OID
		}
	}

	if remoteOID == localOID {
		fmt.Printf("Everything up-to-date (%s)\n", ref)
		return nil
	}
	// Refuse anything but a fast-forward.
	if remoteOID != transport.ZeroOID {
		ok, err := isAncestor(r, remoteOID, localOID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("push: non-fast-forward update rejected for %s", ref)
		}
	}

	reachable, err := object.CollectReachable(r.GitDir, []string{localOID}, have)
	if err != nil {
		return err
	}
	packObjs := make([]transport.PackObject, 0, len(reachable))
	for _, ro := range reachable {
		packObjs = append(packObjs, transport.PackObject{Type: ro.Type, Payload: ro.Payload})
	}
	pack, err := transport.BuildPack(packObjs)
	if err != nil {
		return err
	}

	update := transport.RefUpdate{Old: remoteOID, New: localOID, Ref: ref}
	report, err := transport.ReceivePack(client, cleanURL, auth, []transport.RefUpdate{update}, pack)
	if err != nil {
		return err
	}
	fmt.Printf("Pushed %d object(s) to %s\n", len(packObjs), cleanURL)
	if strings.TrimSpace(report) != "" {
		fmt.Print(report)
		if !strings.HasSuffix(report, "\n") {
			fmt.Println()
		}
	}
	return nil
}

// splitAuth takes credentials from the URL (user:token@host) or, failing that,
// from GG_USER and GG_TOKEN. It returns the URL with any credentials removed.
func splitAuth(remoteURL string) (string, transport.Auth) {
	auth := transport.Auth{
		User:  os.Getenv("GG_USER"),
		Token: os.Getenv("GG_TOKEN"),
	}
	schemeIdx := strings.Index(remoteURL, "://")
	if schemeIdx < 0 {
		return remoteURL, auth
	}
	scheme := remoteURL[:schemeIdx+3]
	rest := remoteURL[schemeIdx+3:]
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		creds := rest[:at]
		rest = rest[at+1:]
		if colon := strings.Index(creds, ":"); colon >= 0 {
			auth.User = creds[:colon]
			auth.Token = creds[colon+1:]
		} else {
			auth.User = creds
		}
	}
	return scheme + rest, auth
}

// isAncestor reports whether maybe is an ancestor of (or equal to) tip.
func isAncestor(r *repo.Repo, maybe, tip string) (bool, error) {
	seen := map[string]bool{}
	stack := []string{tip}
	for len(stack) > 0 {
		oid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if oid == maybe {
			return true, nil
		}
		if seen[oid] {
			continue
		}
		seen[oid] = true
		t, payload, err := object.Read(r.GitDir, oid)
		if err != nil {
			// We don't have the object, so we can't prove ancestry.
			return false, nil
		}
		if t != object.TypeCommit {
			continue
		}
		c, err := object.DecodeCommit(payload)
		if err != nil {
			return false, err
		}
		stack = append(stack, c.Parents...)
	}
	return false, nil
}
