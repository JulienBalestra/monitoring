package etcd

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
)

const (
	CollectorName = "wireguard-stun-registry-etcd"
)

type Collector struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewWireguardStunRegistryEtcd(conf *collector.Config) collector.Collector {
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
	return c.exporter.Tags()
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{
		exporter.OptionURL:                             "http://127.0.0.1:8989/metrics",
		"wireguard_stun_peers":                         "wireguard_stun.peers",
		"wireguard_stun_registry_etcd_txn":             "wireguard_stun.registry.etcd.txn",
		"wireguard_stun_registry_etcd_update_triggers": "wireguard_stun.registry.etcd.updates",
		"wireguard_stun_etcd_conn_state":               "wireguard_stun.etcd.conn.state",
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
