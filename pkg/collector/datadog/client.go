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
)

type Datadog struct {
	conf *collector.Config

	ClientMetrics *datadog.ClientMetrics
}

func NewDatadogReporter(conf *collector.Config) *Datadog {
	return &Datadog{
		conf: conf,
	}
}

func (c *Datadog) Config() *collector.Config {
	return c.conf
}

func (c *Datadog) Name() string {
	return CollectorName
}

func (c *Datadog) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
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
	}, gauges, nil
}
