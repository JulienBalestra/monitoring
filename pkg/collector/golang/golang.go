package golang

import (
	"context"
	"runtime"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "golang"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewGolang(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	now, tags := time.Now(), c.Tags()

	memstat := &runtime.MemStats{}
	runtime.ReadMemStats(memstat)

	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "golang.runtime.goroutines",
		Value: float64(runtime.NumGoroutine()),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "golang.heap.alloc",
		Value: float64(memstat.HeapAlloc),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)

	return nil
}
