package uptime

import (
	"context"
	"syscall"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorUptimeName = "uptime"
)

type Uptime struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewUptime(conf *collector.Config) collector.Collector {
	return &Uptime{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Uptime) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Uptime) DefaultCollectInterval() time.Duration {
	return time.Minute * 5
}

func (c *Uptime) Config() *collector.Config {
	return c.conf
}

func (c *Uptime) IsDaemon() bool { return false }

func (c *Uptime) Name() string {
	return CollectorUptimeName
}

func (c *Uptime) Collect(_ context.Context) error {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return err
	}

	c.measures.Gauge(&metrics.Sample{
		Name:      "uptime.seconds",
		Value:     float64(info.Uptime),
		Timestamp: time.Now(),
		Host:      c.conf.Host,
		Tags:      c.conf.Tagger.GetUnstable(c.conf.Host),
	})
	return nil
}
