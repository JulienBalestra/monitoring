package datadog

import (
	"math"
	"time"
)

const (
	typeCount = "count"
	typeGauge = "gauge"
)

type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Host      string

	Tags []string
}

type GaugeList []*Metric

func (m *GaugeList) Gauge(chanSeries chan Series) {
	for _, metric := range *m {
		chanSeries <- Series{
			Metric: metric.Name,
			Points: [][]float64{
				{float64(metric.Timestamp.Unix()), metric.Value},
			},
			Type: typeGauge,
			Host: metric.Host,
			Tags: metric.Tags,
		}
	}
}

type CounterMap map[string]*Metric

func (m *CounterMap) Count(chanSeries chan Series, newMetrics CounterMap) {
	for path, prevMetric := range *m {
		newMetric, ok := newMetrics[path]
		if !ok {
			continue
		}
		metricsValue := newMetric.Value - prevMetric.Value
		// count must be > 0
		if metricsValue <= 0 {
			continue
		}
		chanSeries <- Series{
			Metric: newMetric.Name,
			Points: [][]float64{
				{float64(prevMetric.Timestamp.Unix()), metricsValue},
			},
			Type:     typeCount,
			Interval: math.Round(newMetric.Timestamp.Sub(prevMetric.Timestamp).Seconds()),
			Host:     newMetric.Host,
			Tags:     newMetric.Tags,
		}
	}
}
