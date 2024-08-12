package cli

// Usage is printed for --help and after usage errors.
const Usage = `ccmc - a memcached text-protocol CLI

Usage:
  ccmc [global flags] [command [args...]]

If no command is given, ccmc starts an interactive REPL against the connection.

Global flags:
  -host string   memcached host (default "localhost")
  -p int         memcached port (default 11211)
  -t duration    dial/read timeout (default 5s)
  -flags uint    flags value for storage commands (default 0)
  -exptime int   expiration seconds for storage commands (default 0)
  -noreply       send storage/delete with noreply (no server ack)
  -q             quiet: print only the raw value(s) for get
  -h, --help, -?, --?   show this help and exit

Commands:
  set     <key> [value]        store (value from argv or stdin)
  add     <key> [value]        store only if absent
  replace <key> [value]        store only if present
  append  <key> [value]        append to an existing value
  prepend <key> [value]        prepend to an existing value
  get     <key> [key...]       retrieve one or more keys
  gets    <key> [key...]       retrieve with CAS unique
  cas     <key> <casid> [val]  check-and-set
  delete  <key>                remove a key
  incr    <key> <delta>        increment a numeric value
  decr    <key> <delta>        decrement a numeric value
  stats   [arg]                server statistics

Examples:
  ccmc -host server.com -p 12121 set testkey testvalue
  ccmc get testkey
  printf 'blob' | ccmc set bigkey
  ccmc stats

Exit codes:
  0 success   1 logical miss (not found / NOT_STORED / EXISTS)
  2 usage     3 connection or protocol error
`

// ReplHelp returns the short help shown in the REPL.
func ReplHelp() string {
	return `Commands: set add replace append prepend get gets cas delete incr decr stats
Type a command and its arguments, e.g. "set foo bar" or "get foo".
'quit' or 'exit' to leave.
`
}
