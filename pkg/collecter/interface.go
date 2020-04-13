package collecter

import (
	"context"
	"time"

	"github.com/JulienBalestra/metrics/pkg/datadog"
)

type Config struct {
	MetricsCh chan datadog.Series
	Tagger    *datadog.Tagger

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
