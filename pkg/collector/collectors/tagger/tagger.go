package tagger

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "tagger"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewTagger(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	})
}

func (c *Collector) SubmittedSeries() float64 {
	return c.measures.GetTotalSubmittedSeries()
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	now := time.Now()
	tags := c.conf.Tagger.GetUnstable(c.conf.Host)

	entities, keys, tagsNumber := c.conf.Tagger.Stats()
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "tagger.entities",
		Value: entities,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "tagger.keys",
		Value: keys,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "tagger.tags",
		Value: tagsNumber,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	return nil
}
