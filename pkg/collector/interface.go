package collector

import (
	"context"
	"log"
	"time"

	"github.com/JulienBalestra/metrics/pkg/metrics"

	"github.com/JulienBalestra/metrics/pkg/tagger"
)

type Config struct {
	SeriesCh chan metrics.Series
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
	Collect(context.Context) error
	Name() string
	IsDaemon() bool
}

func RunCollection(ctx context.Context, c Collector) {
	config := c.Config()

	if c.IsDaemon() {
		log.Printf("collecting metrics continuously: %s", c.Name())
		err := c.Collect(ctx)
		if err != nil {
			// TODO manage this ?
		}
		return
	}

	ticker := time.NewTicker(config.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting metrics every %s: %s", config.CollectInterval.String(), c.Name())

	for {
		select {
		case <-ctx.Done():
			log.Printf("end of collection: %s", c.Name())
			return

		case <-ticker.C:
			err := c.Collect(ctx)
			if err != nil {
				log.Printf("failed collection: %s: %v", c.Name(), err)
				continue
			}
			log.Printf("successfully run collection: %s", c.Name())
		}
	}
}
