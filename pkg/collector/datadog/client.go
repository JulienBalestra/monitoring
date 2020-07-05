package datadog

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "datadog-client"

	clientMetricPrefix = "client."

	clientSentByteMetric   = clientMetricPrefix + "sent.bytes"
	clientSentSeriesMetric = clientMetricPrefix + "sent.series"

	clientErrorsMetric = clientMetricPrefix + "errors"
	clientStoreMetric  = clientMetricPrefix + "store.aggregations"
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
			Name:      clientSentByteMetric,
			Value:     c.conf.MetricsClient.Stats.SentSeriesBytes,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientSentSeriesMetric,
			Value:     c.conf.MetricsClient.Stats.SentSeries,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientErrorsMetric,
			Value:     c.conf.MetricsClient.Stats.SentSeriesErrors,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientStoreMetric,
			Value:     c.conf.MetricsClient.Stats.StoreAggregations,
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
