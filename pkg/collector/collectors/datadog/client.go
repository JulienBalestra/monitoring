package datadog

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "datadog-client"

	clientPrefix = "client."

	// metrics
	clientSentByteMetrics   = clientPrefix + "sent.metrics.bytes"
	clientSentSeriesMetrics = clientPrefix + "sent.metrics.series"

	clientSentSeriesErrors         = clientPrefix + "metrics.errors"
	clientMetricsStoreAggregations = clientPrefix + "metrics.store.aggregations"

	// logs
	clientSentLogsBytes = clientPrefix + "sent.logs.bytes"
	SentLogsErrors      = clientPrefix + "logs.errors"
)

type Collector struct {
	conf *collector.Config

	measures *metrics.Measures
}

func NewClient(conf *collector.Config) collector.Collector {
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

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	now := time.Now()
	tags := c.Tags()
	c.conf.MetricsClient.Stats.RLock()
	samples := []*metrics.Sample{
		{
			Name:  clientSentByteMetrics,
			Value: c.conf.MetricsClient.Stats.SentSeriesBytes,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
		{
			Name:  clientSentSeriesMetrics,
			Value: c.conf.MetricsClient.Stats.SentSeries,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
		{
			Name:  clientSentSeriesErrors,
			Value: c.conf.MetricsClient.Stats.SentSeriesErrors,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
		{
			Name:  clientMetricsStoreAggregations,
			Value: c.conf.MetricsClient.Stats.StoreAggregations,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
		{
			Name:  SentLogsErrors,
			Value: c.conf.MetricsClient.Stats.SentLogsErrors,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
		{
			Name:  clientSentLogsBytes,
			Value: c.conf.MetricsClient.Stats.SentLogsBytes,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  tags,
		},
	}
	c.conf.MetricsClient.Stats.RUnlock()
	for _, s := range samples {
		_ = c.measures.Count(s)
	}
	return nil
}
