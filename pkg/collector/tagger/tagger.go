package tagger

import (
	"context"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/datadog"
)

const (
	CollectorName = "tagger"
)

type Tagger struct {
	conf *collector.Config
}

func NewTagger(conf *collector.Config) collector.Collector {
	return &Tagger{
		conf: conf,
	}
}

func (c *Tagger) Config() *collector.Config {
	return c.conf
}

func (c *Tagger) IsDaemon() bool { return false }

func (c *Tagger) Name() string {
	return CollectorName
}

func (c *Tagger) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	now := time.Now()
	tags := c.conf.Tagger.Get(c.conf.Host)

	entities, keys, tagsNumber := c.conf.Tagger.Stats()
	gauges = append(gauges,
		&datadog.Metric{
			Name:      "tagger.entities",
			Value:     entities,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		},
		&datadog.Metric{
			Name:      "tagger.keys",
			Value:     keys,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		},
		&datadog.Metric{
			Name:      "tagger.tags",
			Value:     tagsNumber,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		},
	)
	return counters, gauges, nil
}
