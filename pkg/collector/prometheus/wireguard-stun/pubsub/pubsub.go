package pubsub

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
)

const (
	CollectorName = "wireguard-stun-pubsub"
)

type Collector struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewPubSub(conf *collector.Config) collector.Collector {
	return &Collector{
		conf: conf,

		exporter: exporter.NewPrometheusExporter(conf),
	}
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
		exporter.OptionURL:                           "http://127.0.0.1:8989/metrics",
		"wireguard_stun_pubsub_active_peers":         "wireguard_stun.pubsub.active.peers",
		"wireguard_stun_pubsub_active_subscriptions": "wireguard_stun.pubsub.active.subscriptions",
		"wireguard_stun_pubsub_new_subscriptions":    "wireguard_stun.pubsub.new.subscriptions",
		"wireguard_stun_pubsub_sent_events":          "wireguard_stun.pubsub.sent.events",
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
