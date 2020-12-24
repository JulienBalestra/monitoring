package pubsub

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
)

const (
	CollectorName = "wireguard-stun-pubsub"

	optionURL = "exporter-url"
)

type PubSub struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewPubSub(conf *collector.Config) collector.Collector {
	return &PubSub{
		conf: conf,

		exporter: exporter.NewPrometheusExporter(conf),
	}
}

func (c *PubSub) DefaultOptions() map[string]string {
	return map[string]string{
		optionURL:                                    "http://127.0.0.1:8989/metrics",
		"wireguard_stun_pubsub_active_peers":         "wireguard_stun.pubsub.active.peers",
		"wireguard_stun_pubsub_active_subscriptions": "wireguard_stun.pubsub.active.subscriptions",
		"wireguard_stun_pubsub_new_subscriptions":    "wireguard_stun.pubsub.new.subscriptions",
		"wireguard_stun_pubsub_sent_events":          "wireguard_stun.pubsub.sent.events",
	}
}

func (c *PubSub) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *PubSub) Config() *collector.Config {
	return c.conf
}

func (c *PubSub) IsDaemon() bool { return false }

func (c *PubSub) Name() string {
	return CollectorName
}

func (c *PubSub) Collect(ctx context.Context) error {
	return c.exporter.Collect(ctx)
}
