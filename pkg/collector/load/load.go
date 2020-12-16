package load

import (
	"context"
	"math"
	"syscall"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorLoadName = "load"
)

type Load struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewLoad(conf *collector.Config) collector.Collector {
	return &Load{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Load) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Load) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Load) Config() *collector.Config {
	return c.conf
}

func (c *Load) IsDaemon() bool { return false }

func (c *Load) Name() string {
	return CollectorLoadName
}

func formatLoad(f uint64) float64 {
	v := float64(f) / (1 << 16.)
	v *= 100
	v = math.Round(v)
	return v / 100
}

func (c *Load) Collect(_ context.Context) error {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return err
	}

	now, tags := time.Now(), c.conf.Tagger.GetUnstable(c.conf.Host)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.1",
		Value:     formatLoad(info.Loads[0]),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.5",
		Value:     formatLoad(info.Loads[1]),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.15",
		Value:     formatLoad(info.Loads[2]),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	return nil
}
