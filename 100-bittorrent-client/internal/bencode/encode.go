package bencode

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
)

// Encode returns the bencoding of v. Dictionary keys are written in sorted
// byte order, as the format requires.
//
// Supported types:
//
//	string, []byte                 -> bencode string
//	int, int8..int64, uint..uint64 -> bencode integer
//	[]interface{}                  -> bencode list
//	map[string]interface{}         -> bencode dictionary
//	bool                           -> bencode integer (0/1)
func Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(buf *bytes.Buffer, v interface{}) error {
	switch x := v.(type) {
	case string:
		encodeString(buf, []byte(x))
	case []byte:
		encodeString(buf, x)
	case int:
		encodeInt(buf, int64(x))
	case int8:
		encodeInt(buf, int64(x))
	case int16:
		encodeInt(buf, int64(x))
	case int32:
		encodeInt(buf, int64(x))
	case int64:
		encodeInt(buf, x)
	case uint:
		encodeInt(buf, int64(x))
	case uint8:
		encodeInt(buf, int64(x))
	case uint16:
		encodeInt(buf, int64(x))
	case uint32:
		encodeInt(buf, int64(x))
	case uint64:
		encodeInt(buf, int64(x))
	case bool:
		if x {
			encodeInt(buf, 1)
		} else {
			encodeInt(buf, 0)
		}
	case []interface{}:
		buf.WriteByte('l')
		for _, e := range x {
			if err := encodeValue(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	case map[string]interface{}:
		buf.WriteByte('d')
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			encodeString(buf, []byte(k))
			if err := encodeValue(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	default:
		return fmt.Errorf("bencode: cannot encode value of type %T", v)
	}
	return nil
}

func encodeString(buf *bytes.Buffer, b []byte) {
	buf.WriteString(strconv.Itoa(len(b)))
	buf.WriteByte(':')
	buf.Write(b)
}

func encodeInt(buf *bytes.Buffer, n int64) {
	buf.WriteByte('i')
	buf.WriteString(strconv.FormatInt(n, 10))
	buf.WriteByte('e')
}
