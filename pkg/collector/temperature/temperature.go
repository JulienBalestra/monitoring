package temperature

import (
	"context"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/JulienBalestra/metrics/pkg/metrics"

	"github.com/JulienBalestra/metrics/pkg/collector"
)

const (
	CollectorTemperatureName = "temperature"

	cpuTemperaturePath = "/proc/dmu/temperature"
)

type Temperature struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewTemperature(conf *collector.Config) collector.Collector {
	return &Temperature{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
	}
}

func (c *Temperature) Config() *collector.Config {
	return c.conf
}

func (c *Temperature) IsDaemon() bool { return false }

func (c *Temperature) Name() string {
	return CollectorTemperatureName
}

func (c *Temperature) Collect(_ context.Context) error {
	// example content:
	// 669
	load, err := ioutil.ReadFile(cpuTemperaturePath)
	if err != nil {
		return err
	}

	t, err := strconv.ParseFloat(string(load[:len(load)-1]), 10)
	if err != nil {
		return err
	}
	t /= 10

	c.measures.Gauge(&metrics.Sample{
		Name:      "cpu.temperature",
		Value:     t,
		Timestamp: time.Now(),
		Host:      c.conf.Host,
		Tags:      c.conf.Tagger.Get(c.conf.Host),
	},
	)
	return nil
}
