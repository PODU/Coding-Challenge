// Command api consumes stories from the broker into an in-memory store and
// serves them over a REST API plus an auto-refreshing web frontend.
//
// With -broker=memory (the default offline mode) it also runs an in-process
// scraper+producer feeding the same bus, so the dashboard works without Kafka.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codingchallenges/topstories/internal/broker"
	"github.com/codingchallenges/topstories/internal/broker/kafka"
	"github.com/codingchallenges/topstories/internal/broker/memory"
	"github.com/codingchallenges/topstories/internal/config"
	"github.com/codingchallenges/topstories/internal/httpapi"
	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/pipeline"
	"github.com/codingchallenges/topstories/internal/ranking"
	"github.com/codingchallenges/topstories/internal/source"
	"github.com/codingchallenges/topstories/internal/source/hackernews"
	"github.com/codingchallenges/topstories/internal/store"
	"github.com/codingchallenges/topstories/web"
)

func main() {
	var (
		cfgPath    = flag.String("config", "config.yaml", "path to config.yaml")
		brokerKind = flag.String("broker", "memory", "broker: memory|kafka")
		addr       = flag.String("addr", ":8080", "HTTP listen address")
		group      = flag.String("group", "api-consumer", "kafka consumer group")
		scrape     = flag.Bool("scrape", true, "run an in-process scraper (memory broker only)")
	)
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.New()

	filter := ranking.Filter{
		MinScore: cfg.Filter.MinScore,
		Types:    cfg.Filter.Types,
		Keywords: cfg.Filter.Keywords,
	}

	var consumer broker.Consumer
	var memBus *memory.Bus
	switch *brokerKind {
	case "kafka":
		consumer = kafka.NewConsumer(cfg.Brokers, cfg.Topic, *group, log)
	default:
		memBus = memory.New(1024)
		consumer = memBus
		log.Warn("using in-memory broker (offline mode)")
	}

	go func() {
		err := consumer.Consume(ctx, func(s model.Story) error {
			if len(ranking.Apply([]model.Story{s}, filter)) == 0 {
				return nil
			}
			st.Upsert(s)
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("consumer stopped", "err", err)
		}
	}()

	// With the memory broker nothing else feeds the bus, so scrape in-process.
	if memBus != nil && *scrape {
		go runScraper(ctx, cfg, memBus, log)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(st, web.FS),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", *addr, "broker", *brokerKind)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down api")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	_ = consumer.Close()
}

// runScraper periodically scrapes HN and publishes to bus, standing in for the
// producer binary when running without Kafka.
func runScraper(ctx context.Context, cfg config.Config, bus *memory.Bus, log *slog.Logger) {
	src := hackernews.New()
	agg := pipeline.New(src, 16, pipeline.WithMaxPerCategory(100), pipeline.WithLogger(log))
	cats := make([]source.Category, 0, len(cfg.Categories))
	for _, c := range cfg.Categories {
		if cat, ok := source.ParseCategory(c); ok {
			cats = append(cats, cat)
		}
	}
	if len(cats) == 0 {
		cats = []source.Category{source.CatTop, source.CatBest}
	}
	dedup := pipeline.NewDeduper()

	scrapeOnce := func() {
		stories, err := agg.Collect(ctx, cats...)
		if err != nil {
			log.Warn("scrape failed", "err", err)
			return
		}
		n := pipeline.PublishAll(ctx, bus, dedup, stories, log)
		log.Info("scraped", "collected", len(stories), "published", n)
	}

	scrapeOnce()
	ticker := time.NewTicker(cfg.PollDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scrapeOnce()
		}
	}
}
