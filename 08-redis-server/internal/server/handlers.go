package server

import (
	"errors"
	"strconv"
	"strings"

	"rediserver/internal/resp"
	"rediserver/internal/store"
)

func cmdPing(_ *store.Store, args []string) resp.Value {
	switch len(args) {
	case 0:
		return resp.SimpleString("PONG")
	case 1:
		return resp.BulkString(args[0])
	default:
		return errWrongArgs("ping")
	}
}

func cmdEcho(_ *store.Store, args []string) resp.Value {
	if len(args) != 1 {
		return errWrongArgs("echo")
	}
	return resp.BulkString(args[0])
}

// cmdSet implements:
//
//	SET key value [EX s | PX ms | EXAT ts-s | PXAT ts-ms] [NX | XX] [KEEPTTL] [GET]
func cmdSet(st *store.Store, args []string) resp.Value {
	if len(args) < 2 {
		return errWrongArgs("set")
	}
	key, value := args[0], args[1]

	var opts store.SetOptions
	var wantGet, haveExpiry bool
	now := st.Now()

	for i := 2; i < len(args); i++ {
		opt := strings.ToUpper(args[i])
		switch opt {
		case "EX", "PX", "EXAT", "PXAT":
			if haveExpiry || i+1 >= len(args) {
				return errSyntax()
			}
			n, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil {
				return errNotInt()
			}
			switch opt {
			case "EX":
				if n <= 0 {
					return resp.ErrorString("ERR invalid expire time in 'set' command")
				}
				opts.ExpiresAt = now + n*1000
			case "PX":
				if n <= 0 {
					return resp.ErrorString("ERR invalid expire time in 'set' command")
				}
				opts.ExpiresAt = now + n
			case "EXAT":
				opts.ExpiresAt = n * 1000
			case "PXAT":
				opts.ExpiresAt = n
			}
			haveExpiry = true
			i++
		case "NX":
			opts.NX = true
		case "XX":
			opts.XX = true
		case "KEEPTTL":
			opts.KeepTTL = true
		case "GET":
			wantGet = true
		default:
			return errSyntax()
		}
	}

	if opts.NX && opts.XX {
		return errSyntax()
	}

	old, hadOld, ok, err := st.Set(key, value, opts, wantGet)
	if err != nil {
		if errors.Is(err, store.ErrWrongType) {
			return errWrongType()
		}
		return resp.ErrorString("ERR " + err.Error())
	}

	if wantGet {
		// The old value is returned even if NX/XX blocked the write.
		if hadOld {
			return resp.BulkString(old)
		}
		return resp.NullBulk()
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.SimpleString("OK")
}

func cmdGet(st *store.Store, args []string) resp.Value {
	if len(args) != 1 {
		return errWrongArgs("get")
	}
	val, ok, err := st.Get(args[0])
	if err != nil {
		return errWrongType()
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.BulkString(val)
}

func cmdExists(st *store.Store, args []string) resp.Value {
	if len(args) < 1 {
		return errWrongArgs("exists")
	}
	return resp.Int(int64(st.Exists(args...)))
}

func cmdDel(st *store.Store, args []string) resp.Value {
	if len(args) < 1 {
		return errWrongArgs("del")
	}
	return resp.Int(int64(st.Del(args...)))
}

func cmdIncr(st *store.Store, args []string) resp.Value {
	if len(args) != 1 {
		return errWrongArgs("incr")
	}
	return incrBy(st, args[0], 1)
}

func cmdDecr(st *store.Store, args []string) resp.Value {
	if len(args) != 1 {
		return errWrongArgs("decr")
	}
	return incrBy(st, args[0], -1)
}

func incrBy(st *store.Store, key string, delta int64) resp.Value {
	n, err := st.IncrBy(key, delta)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrWrongType):
			return errWrongType()
		case errors.Is(err, store.ErrNotInteger):
			return errNotInt()
		case errors.Is(err, store.ErrOverflow):
			return resp.ErrorString("ERR increment or decrement would overflow")
		default:
			return resp.ErrorString("ERR " + err.Error())
		}
	}
	return resp.Int(n)
}

func cmdLPush(st *store.Store, args []string) resp.Value { return push(st, args, true) }
func cmdRPush(st *store.Store, args []string) resp.Value { return push(st, args, false) }

func push(st *store.Store, args []string, left bool) resp.Value {
	name := "rpush"
	if left {
		name = "lpush"
	}
	if len(args) < 2 {
		return errWrongArgs(name)
	}
	n, err := st.Push(args[0], args[1:], left)
	if err != nil {
		if errors.Is(err, store.ErrWrongType) {
			return errWrongType()
		}
		return resp.ErrorString("ERR " + err.Error())
	}
	return resp.Int(int64(n))
}

func cmdLRange(st *store.Store, args []string) resp.Value {
	if len(args) != 3 {
		return errWrongArgs("lrange")
	}
	start, err := strconv.Atoi(args[1])
	if err != nil {
		return errNotInt()
	}
	stop, err := strconv.Atoi(args[2])
	if err != nil {
		return errNotInt()
	}
	items, err := st.LRange(args[0], start, stop)
	if err != nil {
		if errors.Is(err, store.ErrWrongType) {
			return errWrongType()
		}
		return resp.ErrorString("ERR " + err.Error())
	}
	return resp.BulkArray(items)
}

// cmdCommand returns an empty array. redis-cli sends COMMAND DOCS on connect
// and is happy with that.
func cmdCommand(_ *store.Store, _ []string) resp.Value {
	return resp.ArrayOf()
}

func (s *Server) cmdSave(args []string) resp.Value {
	if len(args) != 0 {
		return errWrongArgs("save")
	}
	if err := s.Store.Save(s.DBFile); err != nil {
		return resp.ErrorString("ERR " + err.Error())
	}
	return resp.SimpleString("OK")
}
