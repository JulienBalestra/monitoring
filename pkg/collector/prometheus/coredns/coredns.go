package coredns

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
)

const (
	CollectorName = "coredns"

	optionURL = "exporter-url"
)

type Coredns struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewCoredns(conf *collector.Config) collector.Collector {
	return &Coredns{
		conf: conf,

		exporter: exporter.NewPrometheusExporter(conf),
	}
}

func (c *Coredns) DefaultOptions() map[string]string {
	return map[string]string{
		// https://coredns.io/plugins/metrics
		optionURL:                     "http://127.0.0.1:9153/metrics",
		"coredns_dns_requests_total":  "coredns.dns.requests",
		"coredns_dns_responses_total": "coredns.dns.responses",
	}
}

func (c *Coredns) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Coredns) Config() *collector.Config {
	return c.conf
}

func (c *Coredns) IsDaemon() bool { return false }

func (c *Coredns) Name() string {
	return CollectorName
}

func (c *Coredns) Collect(ctx context.Context) error {
	return c.exporter.Collect(ctx)
}
