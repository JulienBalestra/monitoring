package tagger

import (
	"context"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/metrics"
)

const (
	CollectorName = "tagger"
)

type Tagger struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewTagger(conf *collector.Config) collector.Collector {
	return &Tagger{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
	}
}

func (c *Tagger) Config() *collector.Config {
	return c.conf
}

func (c *Tagger) IsDaemon() bool { return false }

func (c *Tagger) Name() string {
	return CollectorName
}

func (c *Tagger) Collect(_ context.Context) error {
	now := time.Now()
	tags := c.conf.Tagger.Get(c.conf.Host)

	entities, keys, tagsNumber := c.conf.Tagger.Stats()
	c.measures.Gauge(&metrics.Sample{
		Name:      "tagger.entities",
		Value:     entities,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	})
	c.measures.Gauge(&metrics.Sample{
		Name:      "tagger.keys",
		Value:     keys,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	})
	c.measures.Gauge(&metrics.Sample{
		Name:      "tagger.tags",
		Value:     tagsNumber,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	})
	return nil
}
