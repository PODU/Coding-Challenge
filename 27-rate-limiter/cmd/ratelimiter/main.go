// Command ratelimiter serves a demo HTTP API guarded by a configurable rate
// limiter. GET /unlimited is never limited; GET /limited is wrapped by the
// selected algorithm and returns 429 when the client exceeds its allowance.
//
// Run two instances with -backend=redis on different ports and the limit is
// shared between them.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/httpmw"
	"ratelimiter/internal/limiter"
	"ratelimiter/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var (
		addr       = flag.String("addr", "127.0.0.1:8080", "listen address")
		algo       = flag.String("algo", "token-bucket", "algorithm: token-bucket|fixed-window|sliding-log|sliding-counter")
		limit      = flag.Int("limit", 10, "capacity (token bucket) or max requests per window")
		window     = flag.Duration("window", time.Second, "refill reference window or fixed/sliding window size")
		refill     = flag.Float64("refill", 1, "tokens per second (token bucket only)")
		backend    = flag.String("backend", "memory", "state backend: memory|redis")
		redisAddr  = flag.String("redis-addr", "127.0.0.1:6379", "redis address when -backend=redis")
		keyHeader  = flag.String("key-header", "", "trusted header for client IP behind a proxy (else RemoteAddr)")
		failOpen   = flag.Bool("fail-open", true, "on backend error, allow (true) or deny (false)")
		idleTTLArg = flag.Duration("idle-ttl", 10*time.Minute, "evict in-memory keys idle longer than this")
	)
	flag.Parse()

	clk := clock.RealClock{}

	st, err := buildStore(*backend, *redisAddr, *window, *idleTTLArg, clk)
	if err != nil {
		return err
	}
	defer st.Close()

	lim, err := limiter.New(limiter.Algo(*algo), limiter.Config{
		Limit:        *limit,
		Window:       *window,
		RefillPerSec: *refill,
	}, st, clk)
	if err != nil {
		return err
	}

	mw := httpmw.RateLimit(lim, httpmw.Config{
		KeyFn:    httpmw.ClientIPKey(*keyHeader),
		FailOpen: *failOpen,
		Logger:   log.Default(),
	})

	mux := http.NewServeMux()
	mux.Handle("/limited", mw(http.HandlerFunc(limitedHandler)))
	mux.HandleFunc("/unlimited", unlimitedHandler)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("ratelimiter listening on %s (algo=%s limit=%d window=%s backend=%s)",
			*addr, *algo, *limit, *window, *backend)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func buildStore(backend, redisAddr string, window, idleTTL time.Duration, clk clock.Clock) (store.Store, error) {
	switch backend {
	case "memory":
		ms := store.NewMemoryStore(clk, idleTTL)
		ms.StartJanitor(time.Minute)
		return ms, nil
	case "redis":
		client := redis.NewClient(&redis.Options{Addr: redisAddr})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, errors.New("redis ping failed: " + err.Error())
		}
		// Keep keys a few windows past use so Redis cleans up abandoned ones.
		ttl := window * 4
		if ttl < time.Minute {
			ttl = time.Minute
		}
		return store.NewRedisStore(client, "rl:", ttl), nil
	default:
		return nil, errors.New("unknown backend: " + backend)
	}
}

func limitedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Limited, don't over use me!"))
}

func unlimitedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Unlimited! Let's Go!"))
}
