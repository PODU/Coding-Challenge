// Command producer scrapes Hacker News and publishes normalized stories to the
// configured broker (Kafka by default) on a polling loop.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codingchallenges/topstories/internal/broker"
	"github.com/codingchallenges/topstories/internal/broker/kafka"
	"github.com/codingchallenges/topstories/internal/broker/memory"
	"github.com/codingchallenges/topstories/internal/config"
	"github.com/codingchallenges/topstories/internal/pipeline"
	"github.com/codingchallenges/topstories/internal/source"
	"github.com/codingchallenges/topstories/internal/source/hackernews"
)

func main() {
	var (
		cfgPath    = flag.String("config", "config.yaml", "path to config.yaml")
		brokerKind = flag.String("broker", "kafka", "broker: kafka|memory")
		workers    = flag.Int("workers", 16, "fan-out worker pool size")
		maxPerCat  = flag.Int("max-per-category", 100, "max items fetched per category (0=all)")
		once       = flag.Bool("once", false, "run a single scrape then exit")
	)
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	var prod broker.Producer
	switch *brokerKind {
	case "memory":
		prod = memory.New(1024)
		log.Warn("using in-memory broker; messages are not durable and only visible in-process")
	default:
		prod = kafka.NewProducer(cfg.Brokers, cfg.Topic, log)
	}
	defer prod.Close()

	src := hackernews.New()
	agg := pipeline.New(src, *workers, pipeline.WithMaxPerCategory(*maxPerCat), pipeline.WithLogger(log))
	cats := parseCategories(cfg.Categories, log)
	dedup := pipeline.NewDeduper()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run := func() {
		stories, err := agg.Collect(ctx, cats...)
		if err != nil {
			log.Warn("scrape failed", "err", err)
			return
		}
		n := pipeline.PublishAll(ctx, prod, dedup, stories, log)
		log.Info("scrape published", "collected", len(stories), "published", n)
	}

	run()
	if *once {
		return
	}

	ticker := time.NewTicker(cfg.PollDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down producer")
			return
		case <-ticker.C:
			run()
		}
	}
}

func parseCategories(in []string, log *slog.Logger) []source.Category {
	out := make([]source.Category, 0, len(in))
	for _, c := range in {
		if cat, ok := source.ParseCategory(c); ok {
			out = append(out, cat)
		} else {
			log.Warn("ignoring unknown category", "category", c)
		}
	}
	if len(out) == 0 {
		out = []source.Category{source.CatTop, source.CatBest}
	}
	return out
}
