package etcd

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
)

const (
	CollectorName = "etcd"
)

type Collector struct {
	conf *collector.Config

	exporter collector.Collector
}

func NewEtcd(conf *collector.Config) collector.Collector {
	c := exporter.NewPrometheusExporter(conf)
	return &Collector{
		conf: conf,

		exporter: c,
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
		exporter.OptionURL:                        "http://127.0.0.1:2379/metrics",
		"etcd_debugging_mvcc_keys_total":          "etcd.keys",
		"etcd_mvcc_db_total_size_in_bytes":        "etcd.db.global.size",
		"etcd_mvcc_db_total_size_in_use_in_bytes": "etcd.db.use.size",
		"etcd_disk_wal_write_bytes_total":         "etcd.disk.wall.writes",
		"grpc_server_handled_total":               "etcd.grpc.calls",
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
