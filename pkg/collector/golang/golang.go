package golang

import (
	"context"
	"runtime"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorGolangName = "golang"
)

type Golang struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewGolang(conf *collector.Config) collector.Collector {
	return &Golang{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Golang) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Golang) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Golang) Config() *collector.Config {
	return c.conf
}

func (c *Golang) IsDaemon() bool { return false }

func (c *Golang) Name() string {
	return CollectorGolangName
}

func (c *Golang) Collect(_ context.Context) error {
	now, tags := time.Now(), c.conf.Tagger.GetUnstable(c.conf.Host)

	memstat := &runtime.MemStats{}
	runtime.ReadMemStats(memstat)

	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "golang.runtime.goroutines",
		Value:     float64(runtime.NumGoroutine()),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "golang.heap.alloc",
		Value:     float64(memstat.HeapAlloc),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)

	return nil
}
