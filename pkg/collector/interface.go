package collector

import (
	"context"
	"log"
	"time"

	"github.com/JulienBalestra/metrics/pkg/tagger"

	"github.com/JulienBalestra/metrics/pkg/datadog"
)

type Config struct {
	SeriesCh chan datadog.Series
	Tagger   *tagger.Tagger

	Host            string
	CollectInterval time.Duration
}

func (c Config) OverrideCollectInterval(d time.Duration) *Config {
	c.CollectInterval = d
	return &c
}

type Collector interface {
	Config() *Config
	Collect(context.Context) (datadog.Counter, datadog.Gauge, error)
	Name() string
	IsDaemon() bool
}

func RunCollection(ctx context.Context, c Collector) {
	config := c.Config()

	if c.IsDaemon() {
		log.Printf("collecting metrics every %s: %s", config.CollectInterval.String(), c.Name())
		_, _, _ = c.Collect(ctx)
		return
	}

	ticker := time.NewTicker(config.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting metrics every %s: %s", config.CollectInterval.String(), c.Name())

	prevCounters := make(datadog.Counter)
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of collection: %s", c.Name())
			return

		case <-ticker.C:
			newCounters, gauges, err := c.Collect(ctx)
			if err != nil {
				log.Printf("failed collection: %s: %v", c.Name(), err)
				continue
			}
			gauges.Gauge(config.SeriesCh)
			prevCounters = prevCounters.Count(config.SeriesCh, newCounters)
			log.Printf("successfully run collection: %d counters, %d gauges: %s", len(prevCounters), len(gauges), c.Name())
		}
	}
}
