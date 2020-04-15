package collecter

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"time"

	"github.com/JulienBalestra/metrics/pkg/datadog"
)

type Config struct {
	MetricsCh chan datadog.Series
	Tagger    *tagger.Tagger

	Host            string
	CollectInterval time.Duration
}

func (c Config) OverrideCollectInterval(d time.Duration) *Config {
	c.CollectInterval = d
	return &c
}

type Collecter interface {
	Collect(context.Context)
	//Name() string TODO make the interface with less boilerplate
}
