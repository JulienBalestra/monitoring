package load

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"go.uber.org/zap"
)

const (
	CollectorLoadName = "load"

	loadPath = "/proc/loadavg"
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

func (c *Load) Config() *collector.Config {
	return c.conf
}

func (c *Load) IsDaemon() bool { return false }

func (c *Load) Name() string {
	return CollectorLoadName
}

func (c *Load) Collect(_ context.Context) error {
	// example content:
	// 0.65 0.86 0.99 1/737 37114
	load, err := ioutil.ReadFile(loadPath)
	if err != nil {
		zap.L().Error("failed to parse metrics", zap.Error(err))
		return err
	}
	parts := strings.Fields(string(load))
	if len(parts) != 5 {
		return fmt.Errorf("failed to parse %s: %q", parts, string(load))
	}
	load1, err := strconv.ParseFloat(parts[0], 10)
	if err != nil {
		return err
	}
	load5, err := strconv.ParseFloat(parts[1], 10)
	if err != nil {
		return err
	}
	load10, err := strconv.ParseFloat(parts[2], 10)
	if err != nil {
		return err
	}

	// newMetric is a convenient way to DRY the following gauges
	now, tags := time.Now(), c.conf.Tagger.GetUnstable(c.conf.Host)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.1",
		Value:     load1,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.5",
		Value:     load5,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "load.10",
		Value:     load10,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}, c.conf.CollectInterval*3)
	return nil
}
