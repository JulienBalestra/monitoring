package temperature

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"io/ioutil"
	"log"
	"strconv"
	"time"
)

const (
	cpuTemperaturePath = "/proc/dmu/temperature"
)

type Temperature struct {
	conf *collecter.Config
}

func NewTemperatureReporter(conf *collecter.Config) *Temperature {
	return &Temperature{
		conf: conf,
	}
}

func (c *Temperature) collectMetrics() (datadog.GaugeList, error) {
	var gaugeLists datadog.GaugeList

	// example content:
	// 669
	load, err := ioutil.ReadFile(cpuTemperaturePath)
	if err != nil {
		return gaugeLists, err
	}

	t, err := strconv.ParseFloat(string(load[:len(load)-1]), 10)
	if err != nil {
		return gaugeLists, err
	}
	t /= 10

	return append(gaugeLists,
		&datadog.Metric{
			Name:      "cpu.temperature",
			Value:     t,
			Timestamp: time.Now(),
			Host:      c.conf.Host,
			Tags:      c.conf.Tagger.Get(c.conf.Host),
		},
	), nil
}

func (c *Temperature) Collect(ctx context.Context) {
	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting temperature metrics every %s", c.conf.CollectInterval.String())
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of temperature collection")
			return

		case <-ticker.C:
			gauges, err := c.collectMetrics()
			if err != nil {
				log.Printf("failed temperature collection: %v", err)
				continue
			}
			gauges.Gauge(c.conf.MetricsCh)
			log.Printf("successfully run temperature collection")
		}
	}
}
