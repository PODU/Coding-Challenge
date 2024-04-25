// Command ccpm is a command-line password manager. Credentials are encrypted
// at rest under a master password: Argon2id derives an AES-256-GCM key that
// seals the whole vault.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ccpm/internal/crypto"
	"ccpm/internal/genpass"
	"ccpm/internal/session"
	"ccpm/internal/vault"

	"golang.org/x/term"
)

const (
	exitOK     = 0
	exitUser   = 1 // user error
	exitAuth   = 2 // authentication failure
	exitTamper = 3 // tamper / corruption
)

const defaultTimeout = 15 * time.Minute

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitUser
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "init":
		err = cmdInit(rest)
	case "signin":
		err = cmdSignin(rest)
	case "signout":
		err = cmdSignout(rest)
	case "add":
		err = cmdAdd(rest)
	case "get":
		err = cmdGet(rest)
	case "list":
		err = cmdList(rest)
	case "update":
		err = cmdUpdate(rest)
	case "rm":
		err = cmdRm(rest)
	case "gen":
		err = cmdGen(rest)
	case "-h", "--help", "help":
		usage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		return exitUser
	}
	return report(err)
}

// report maps an error to an exit code and prints it.
func report(err error) int {
	if err == nil {
		return exitOK
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	switch {
	case errors.Is(err, crypto.ErrAuth):
		return exitAuth
	case errors.Is(err, vault.ErrCorrupt):
		return exitTamper
	default:
		return exitUser
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ccpm — encrypted command-line password manager

Usage:
  ccpm init    <name>                 Create a new encrypted vault
  ccpm signin  <name>                 Authenticate and open a session
  ccpm signout <name>                 End the session
  ccpm add     <name> --record ID [--username U] [--password P] [--gen] [--notes N]
  ccpm get     <name> --record ID [--show]
  ccpm list    <name>
  ccpm update  <name> --record ID [--username U] [--password P] [--gen] [--notes N]
  ccpm rm      <name> --record ID
  ccpm gen     [--length N] [--no-upper] [--no-lower] [--no-digits] [--no-symbols]

Common flags:
  --vault PATH        Override the vault file location (default <name>.ccpm)
  --timeout DUR       Session TTL (e.g. 15m, 0 to disable caching)

Master password is read without echo, or taken from $CCPM_MASTER for scripting.
`)
}

type commonFlags struct {
	vault   string
	timeout time.Duration
}

func addCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	fs.StringVar(&c.vault, "vault", "", "vault file path (default <name>.ccpm)")
	def := defaultTimeout
	if env := os.Getenv("CCPM_TIMEOUT"); env != "" {
		if d, err := time.ParseDuration(env); err == nil {
			def = d
		}
	}
	fs.DurationVar(&c.timeout, "timeout", def, "session TTL")
	return c
}

func vaultPath(c *commonFlags, name string) string {
	if c.vault != "" {
		return c.vault
	}
	return name + ".ccpm"
}

// nameArg takes the vault name, which has to come before any flags.
func nameArg(fs *flag.FlagSet, args []string) (string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, errors.New("missing vault name")
	}
	return args[0], args[1:], nil
}

func readMaster(confirm bool) ([]byte, error) {
	if env, ok := os.LookupEnv("CCPM_MASTER"); ok {
		return []byte(env), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("no terminal for password entry; set $CCPM_MASTER for scripted use")
	}
	fmt.Fprint(os.Stderr, "Master password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm master password: ")
		pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("reading password: %w", err)
		}
		if string(pw) != string(pw2) {
			return nil, errors.New("passwords do not match")
		}
	}
	return pw, nil
}

// authenticate returns the key and decrypted vault, using the cached session
// key if it still works and asking for the master password otherwise.
func authenticate(c *commonFlags, path string) ([]byte, *vault.Vault, error) {
	if key, err := session.Load(path); err == nil {
		v, derr := vault.Load(path, key)
		if derr == nil {
			return key, v, nil
		}
		// The vault was re-keyed since the session started.
		_ = session.End(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("vault %q does not exist; run `ccpm init` first", path)
		}
		return nil, nil, err
	}
	params, err := vault.ParseHeader(data)
	if err != nil {
		return nil, nil, err
	}
	pw, err := readMaster(false)
	if err != nil {
		return nil, nil, err
	}
	defer crypto.Zero(pw)
	key, err := crypto.DeriveKey(pw, params)
	if err != nil {
		return nil, nil, err
	}
	v, err := vault.Decode(data, key)
	if err != nil {
		return nil, nil, err
	}
	if serr := session.Start(path, key, c.timeout); serr != nil {
		fmt.Fprintln(os.Stderr, "warning: could not start session:", serr)
	}
	return key, v, nil
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	c := addCommon(fs)
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	path := vaultPath(c, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("vault %q already exists; refusing to overwrite", path)
	}

	pw, err := readMaster(true)
	if err != nil {
		return err
	}
	defer crypto.Zero(pw)
	if len(pw) == 0 {
		return errors.New("master password must not be empty")
	}

	v, err := vault.New()
	if err != nil {
		return err
	}
	key, err := crypto.DeriveKey(pw, v.Params())
	if err != nil {
		return err
	}
	defer crypto.Zero(key)
	if err := v.Save(path, key); err != nil {
		return err
	}
	if err := session.Start(path, key, c.timeout); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not start session:", err)
	}
	fmt.Printf("Created encrypted vault %q.\n", path)
	return nil
}

func cmdSignin(args []string) error {
	fs := flag.NewFlagSet("signin", flag.ContinueOnError)
	c := addCommon(fs)
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	path := vaultPath(c, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("vault %q does not exist", path)
		}
		return err
	}
	params, err := vault.ParseHeader(data)
	if err != nil {
		return err
	}
	pw, err := readMaster(false)
	if err != nil {
		return err
	}
	defer crypto.Zero(pw)
	key, err := crypto.DeriveKey(pw, params)
	if err != nil {
		return err
	}
	if _, err := vault.Decode(data, key); err != nil {
		return err
	}
	if err := session.Start(path, key, c.timeout); err != nil {
		return err
	}
	crypto.Zero(key)
	fmt.Printf("Signed in to %q (session valid for %s).\n", path, c.timeout)
	return nil
}

func cmdSignout(args []string) error {
	fs := flag.NewFlagSet("signout", flag.ContinueOnError)
	c := addCommon(fs)
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	path := vaultPath(c, name)
	if err := session.End(path); err != nil {
		return err
	}
	fmt.Printf("Signed out of %q.\n", path)
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	c := addCommon(fs)
	record := fs.String("record", "", "record identifier (required)")
	username := fs.String("username", "", "username")
	password := fs.String("password", "", "password")
	notes := fs.String("notes", "", "notes")
	gen := fs.Bool("gen", false, "generate a strong password")
	length := fs.Int("length", 20, "generated password length")
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *record == "" {
		return errors.New("--record is required")
	}
	path := vaultPath(c, name)
	key, v, err := authenticate(c, path)
	if err != nil {
		return err
	}
	defer crypto.Zero(key)

	pw := *password
	if *gen {
		o := genpass.DefaultOptions()
		o.Length = *length
		pw, err = genpass.Generate(o)
		if err != nil {
			return err
		}
	}
	if err := v.Add(vault.Record{
		Name:     *record,
		Username: *username,
		Password: pw,
		Notes:    *notes,
	}); err != nil {
		if errors.Is(err, vault.ErrExists) {
			return fmt.Errorf("%w; use `ccpm update` to modify it", err)
		}
		return err
	}
	if err := v.Save(path, key); err != nil {
		return err
	}
	fmt.Printf("Added record %q.\n", *record)
	if *gen {
		fmt.Printf("Generated password: %s\n", pw)
	}
	return nil
}

func cmdGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	c := addCommon(fs)
	record := fs.String("record", "", "record identifier (required)")
	show := fs.Bool("show", false, "print the password to stdout")
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *record == "" {
		return errors.New("--record is required")
	}
	path := vaultPath(c, name)
	key, v, err := authenticate(c, path)
	if err != nil {
		return err
	}
	defer crypto.Zero(key)

	r, err := v.Get(*record)
	if err != nil {
		return err
	}
	fmt.Printf("Name:     %s\n", r.Name)
	fmt.Printf("Username: %s\n", r.Username)
	if r.Notes != "" {
		fmt.Printf("Notes:    %s\n", r.Notes)
	}
	if *show {
		fmt.Printf("Password: %s\n", r.Password)
	} else {
		fmt.Println("Password: (hidden; pass --show to reveal)")
	}
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	c := addCommon(fs)
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	path := vaultPath(c, name)
	key, v, err := authenticate(c, path)
	if err != nil {
		return err
	}
	defer crypto.Zero(key)

	names := v.List()
	if len(names) == 0 {
		fmt.Println("(vault is empty)")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	c := addCommon(fs)
	record := fs.String("record", "", "record identifier (required)")
	username := fs.String("username", "", "new username")
	password := fs.String("password", "", "new password")
	notes := fs.String("notes", "", "new notes")
	gen := fs.Bool("gen", false, "generate a new strong password")
	length := fs.Int("length", 20, "generated password length")
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *record == "" {
		return errors.New("--record is required")
	}
	// Track which flags were explicitly set so we only change those fields.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	path := vaultPath(c, name)
	key, v, err := authenticate(c, path)
	if err != nil {
		return err
	}
	defer crypto.Zero(key)

	newPw := *password
	if *gen {
		o := genpass.DefaultOptions()
		o.Length = *length
		newPw, err = genpass.Generate(o)
		if err != nil {
			return err
		}
	}
	err = v.Update(*record, func(r *vault.Record) {
		if set["username"] {
			r.Username = *username
		}
		if set["password"] || *gen {
			r.Password = newPw
		}
		if set["notes"] {
			r.Notes = *notes
		}
	})
	if err != nil {
		return err
	}
	if err := v.Save(path, key); err != nil {
		return err
	}
	fmt.Printf("Updated record %q.\n", *record)
	if *gen {
		fmt.Printf("Generated password: %s\n", newPw)
	}
	return nil
}

func cmdRm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	c := addCommon(fs)
	record := fs.String("record", "", "record identifier (required)")
	name, rest, err := nameArg(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *record == "" {
		return errors.New("--record is required")
	}
	path := vaultPath(c, name)
	key, v, err := authenticate(c, path)
	if err != nil {
		return err
	}
	defer crypto.Zero(key)

	if err := v.Delete(*record); err != nil {
		return err
	}
	if err := v.Save(path, key); err != nil {
		return err
	}
	fmt.Printf("Deleted record %q.\n", *record)
	return nil
}

func cmdGen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	length := fs.Int("length", 20, "password length")
	noUpper := fs.Bool("no-upper", false, "exclude uppercase letters")
	noLower := fs.Bool("no-lower", false, "exclude lowercase letters")
	noDigits := fs.Bool("no-digits", false, "exclude digits")
	noSymbols := fs.Bool("no-symbols", false, "exclude symbols")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o := genpass.Options{
		Length:  *length,
		Lower:   !*noLower,
		Upper:   !*noUpper,
		Digits:  !*noDigits,
		Symbols: !*noSymbols,
	}
	pw, err := genpass.Generate(o)
	if err != nil {
		return err
	}
	fmt.Println(pw)
	fmt.Fprintf(os.Stderr, "entropy: ~%.0f bits\n", genpass.Entropy(o))
	return nil
}
