package etcd

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
	dto "github.com/prometheus/client_model/go"
)

const (
	CollectorName = "etcd"

	metricDiskWallWrites = "etcd_disk_wal_write_bytes_total"
)

type Collector struct {
	conf *collector.Config

	exporter *exporter.Collector
}

func NewEtcd(conf *collector.Config) collector.Collector {
	c := &Collector{
		conf: conf,
	}
	_ = collector.WithDefaults(c)
	c.exporter = exporter.NewPrometheusExporter(conf).(*exporter.Collector)
	c.exporter.AddMappingFunction(metricDiskWallWrites, func(family *dto.MetricFamily) {
		*family.Type = dto.MetricType_COUNTER
		for _, m := range family.Metric {
			m.Counter = &dto.Counter{
				Value: m.Gauge.Value,
			}
			m.Gauge = nil
		}
	})
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
		exporter.OptionURL:                        "http://127.0.0.1:2379/metrics",
		"etcd_debugging_mvcc_keys_total":          "etcd.keys",
		"etcd_mvcc_db_total_size_in_bytes":        "etcd.db.total.size",
		"etcd_mvcc_db_total_size_in_use_in_bytes": "etcd.db.use.size",
		metricDiskWallWrites:                      "etcd.wall.writes",
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
