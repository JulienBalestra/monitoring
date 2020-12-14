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

type Client struct {
	conf *collector.Config

	measures *metrics.Measures
}

func NewClient(conf *collector.Config) collector.Collector {
	return &Client{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Client) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Client) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Client) IsDaemon() bool { return false }

func (c *Client) Config() *collector.Config {
	return c.conf
}

func (c *Client) Name() string {
	return CollectorName
}

func (c *Client) Collect(_ context.Context) error {
	now := time.Now()
	tags := c.conf.Tagger.GetUnstable(c.conf.Host)
	c.conf.MetricsClient.Stats.RLock()
	samples := []*metrics.Sample{
		{
			Name:      clientSentByteMetrics,
			Value:     c.conf.MetricsClient.Stats.SentSeriesBytes,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientSentSeriesMetrics,
			Value:     c.conf.MetricsClient.Stats.SentSeries,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientSentSeriesErrors,
			Value:     c.conf.MetricsClient.Stats.SentSeriesErrors,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientMetricsStoreAggregations,
			Value:     c.conf.MetricsClient.Stats.StoreAggregations,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      SentLogsErrors,
			Value:     c.conf.MetricsClient.Stats.SentLogsErrors,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientSentLogsBytes,
			Value:     c.conf.MetricsClient.Stats.SentLogsBytes,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
	}
	c.conf.MetricsClient.Stats.RUnlock()
	for _, s := range samples {
		_ = c.measures.Count(s)
	}
	return nil
}
