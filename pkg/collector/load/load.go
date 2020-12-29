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
	return CollectorName
}

func formatLoad(f float64) float64 {
	v := f / (1 << 16.)
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
		Name:  "load.1",
		Value: formatLoad(float64(info.Loads[0])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "load.5",
		Value: formatLoad(float64(info.Loads[1])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "load.15",
		Value: formatLoad(float64(info.Loads[2])),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*3)
	return nil
}
