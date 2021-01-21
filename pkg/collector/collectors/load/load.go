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
	CollectorName = "load"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewLoad(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	})
}

func (c *Collector) SubmittedSeries() float64 {
	return c.measures.GetTotalSubmittedSeries()
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
	return map[string]string{}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 15
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func formatLoad(f float64) float64 {
	v := f / (1 << 16.)
	v *= 100
	v = math.Round(v)
	return v / 100
}

func (c *Collector) Collect(_ context.Context) error {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return err
	}

	now, tags := time.Now(), c.Tags()
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "load.1",
		Value: formatLoad(float64(info.Loads[0])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "load.5",
		Value: formatLoad(float64(info.Loads[1])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "load.15",
		Value: formatLoad(float64(info.Loads[2])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	return nil
}
