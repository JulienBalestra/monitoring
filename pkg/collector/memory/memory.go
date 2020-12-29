package memory

import (
	"context"
	"syscall"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "memory"
)

type Memory struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewMemory(conf *collector.Config) collector.Collector {
	return &Memory{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Memory) Config() *collector.Config {
	return c.conf
}

func (c *Memory) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Memory) DefaultCollectInterval() time.Duration {
	return time.Second * 60
}

func (c *Memory) IsDaemon() bool { return false }

func (c *Memory) Name() string {
	return CollectorName
}

func (c *Memory) Collect(_ context.Context) error {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return err
	}

	now := time.Now()
	hostTags := c.conf.Tagger.GetUnstable(c.conf.Host)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.total",
		Value: float64(info.Totalram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, time.Minute*5)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.free",
		Value: float64(info.Freeram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.shared",
		Value: float64(info.Sharedram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.buffer",
		Value: float64(info.Bufferram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.swap.total",
		Value: float64(info.Totalswap),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, time.Minute*5)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.swap.free",
		Value: float64(info.Freeswap),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  hostTags,
	}, c.conf.CollectInterval*3)
	return nil
}
