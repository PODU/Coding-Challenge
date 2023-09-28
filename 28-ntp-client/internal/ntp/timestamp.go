package ntp

import "time"

// ntpEpochOffset is the number of seconds from 1900-01-01 (the NTP epoch) to
// 1970-01-01.
const ntpEpochOffset = 2208988800

// Timestamp is an NTP timestamp: seconds since 1900 in the high 32 bits and a
// binary fraction of a second in the low 32.
type Timestamp uint64

// Seconds returns the whole seconds since 1900.
func (ts Timestamp) Seconds() uint32 { return uint32(ts >> 32) }

// Fraction returns the fractional second, in units of 2^-32 s.
func (ts Timestamp) Fraction() uint32 { return uint32(ts & 0xffffffff) }

// TimestampFromTime converts t to an NTP timestamp. The zero time maps to 0,
// which NTP uses for "unset".
func TimestampFromTime(t time.Time) Timestamp {
	if t.IsZero() {
		return 0
	}
	secs := uint64(t.Unix() + ntpEpochOffset)
	nanos := uint64(t.Nanosecond())
	frac := (nanos << 32) / 1000000000
	return Timestamp(secs<<32 | frac)
}

// Time converts ts to a UTC time. 0 maps to the zero time.
//
// The seconds field wraps in 2036; this assumes we are still before that.
func (ts Timestamp) Time() time.Time {
	if ts == 0 {
		return time.Time{}
	}
	secs := int64(ts.Seconds()) - ntpEpochOffset
	nanos := (uint64(ts.Fraction()) * 1000000000) >> 32
	return time.Unix(secs, int64(nanos)).UTC()
}
