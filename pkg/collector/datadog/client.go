package datadog

import (
	"context"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/datadog"
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
}

func NewClient(conf *collector.Config) *Client {
	return &Client{
		conf: conf,
	}
}

func (c *Client) IsDaemon() bool { return false }

func (c *Client) Config() *collector.Config {
	return c.conf
}

func (c *Client) Name() string {
	return CollectorName
}

func (c *Client) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var gauges datadog.Gauge

	now := time.Now()
	tags := c.conf.Tagger.Get(c.conf.Host)
	c.ClientMetrics.RLock()
	defer c.ClientMetrics.RUnlock()
	return datadog.Counter{
		clientSentByteMetric: &datadog.Metric{
			Name:      clientSentByteMetric,
			Value:     c.ClientMetrics.SentBytes,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		clientSentSeriesMetric: &datadog.Metric{
			Name:      clientSentSeriesMetric,
			Value:     c.ClientMetrics.SentSeries,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		clientErrorsMetric: &datadog.Metric{
			Name:      clientErrorsMetric,
			Value:     c.ClientMetrics.SentErrors,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
		clientStoreMetric: &datadog.Metric{
			Name:      clientStoreMetric,
			Value:     c.ClientMetrics.StoreAggregations,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      tags,
		},
	}, gauges, nil
}
