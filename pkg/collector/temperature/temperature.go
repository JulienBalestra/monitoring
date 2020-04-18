package temperature

import (
	"context"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/datadog"
)

const (
	CollectorTemperatureName = "temperature"

	cpuTemperaturePath = "/proc/dmu/temperature"
)

type Temperature struct {
	conf *collector.Config
}

func NewTemperatureReporter(conf *collector.Config) collector.Collector {
	return &Temperature{
		conf: conf,
	}
}

func (c *Temperature) Config() *collector.Config {
	return c.conf
}

func (c *Temperature) Name() string {
	return CollectorTemperatureName
}

func (c *Temperature) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	// example content:
	// 669
	load, err := ioutil.ReadFile(cpuTemperaturePath)
	if err != nil {
		return counters, gauges, err
	}

	t, err := strconv.ParseFloat(string(load[:len(load)-1]), 10)
	if err != nil {
		return counters, gauges, err
	}
	t /= 10

	return counters, append(gauges,
		&datadog.Metric{
			Name:      "cpu.temperature",
			Value:     t,
			Timestamp: time.Now(),
			Host:      c.conf.Host,
			Tags:      c.conf.Tagger.Get(c.conf.Host),
		},
	), nil
}
