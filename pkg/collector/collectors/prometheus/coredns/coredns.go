package coredns

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/prometheus/exporter"
)

const (
	CollectorName = "coredns"
)

type Collector struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewCoredns(conf *collector.Config) collector.Collector {
	c := &Collector{
		conf: conf,
	}
	_ = collector.WithDefaults(c)
	c.exporter = exporter.NewPrometheusExporter(conf)
	return c
}

func (c *Collector) SubmittedSeries() float64 {
	return c.exporter.SubmittedSeries()
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
	return map[string]string{
		// https://coredns.io/plugins/metrics
		exporter.OptionURL:            "http://127.0.0.1:9153/metrics",
		"coredns_dns_requests_total":  "coredns.dns.requests",
		"coredns_dns_responses_total": "coredns.dns.responses",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(ctx context.Context) error {
	return c.exporter.Collect(ctx)
}
