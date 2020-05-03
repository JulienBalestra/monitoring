package datadog

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/datadog"
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

	ClientMetrics *datadog.ClientMetrics
	measures      *metrics.Measures
}

func NewClient(conf *collector.Config) *Client {
	return &Client{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
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
	c.ClientMetrics.RLock()
	samples := []*metrics.Sample{
		{
			Name:      clientSentByteMetric,
			Value:     c.ClientMetrics.SentBytes,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientSentSeriesMetric,
			Value:     c.ClientMetrics.SentSeries,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientErrorsMetric,
			Value:     c.ClientMetrics.SentErrors,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		{
			Name:      clientStoreMetric,
			Value:     c.ClientMetrics.StoreAggregations,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
	}
	c.ClientMetrics.RUnlock()
	for _, s := range samples {
		_ = c.measures.Count(s)
	}
	return nil
}
