package ntp

import (
	"os"
	"testing"
	"time"
)

// TestLiveQuery performs a real UDP exchange with pool.ntp.org. It is GATED
// behind the NTP_LIVE_TEST environment variable so the default `go test` run
// stays fully offline and green.
//
// Enable with:  NTP_LIVE_TEST=1 go test ./...
func TestLiveQuery(t *testing.T) {
	if os.Getenv("NTP_LIVE_TEST") == "" {
		t.Skip("set NTP_LIVE_TEST=1 to run the live pool.ntp.org query")
	}

	res, err := QueryBest(DefaultServer, 3, Options{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("live query failed: %v", err)
	}

	if res.Stratum < 1 || res.Stratum > MaxStratum {
		t.Errorf("stratum = %d, want 1..15", res.Stratum)
	}

	// Sanity: the local clock should be within a loose bound of NTP time.
	const bound = 5 * time.Second
	if res.Offset > bound || res.Offset < -bound {
		t.Errorf("offset = %v, outside +/-%v of local clock", res.Offset, bound)
	}
	if res.Delay < 0 {
		t.Errorf("delay = %v, want non-negative", res.Delay)
	}

	t.Logf("server=%s stratum=%d offset=%v delay=%v ntp=%s",
		res.Server, res.Stratum, res.Offset, res.Delay, res.T2.Format(time.RFC3339Nano))
}
