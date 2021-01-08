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

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewMemory(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
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

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 60
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return err
	}

	now := time.Now()
	tags := c.Tags()
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.total",
		Value: float64(info.Totalram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute*5)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.free",
		Value: float64(info.Freeram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.shared",
		Value: float64(info.Sharedram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.buffer",
		Value: float64(info.Bufferram),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.swap.total",
		Value: float64(info.Totalswap),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute*5)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.swap.free",
		Value: float64(info.Freeswap),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	return nil
}
