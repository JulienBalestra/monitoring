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

type Gauge []*Metric

func (m *Gauge) Gauge(chanSeries chan Series) {
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

func (m *Gauge) GetSeries() []Series {
	var series []Series
	for _, metric := range *m {
		series = append(series, Series{
			Metric: metric.Name,
			Points: [][]float64{
				{float64(metric.Timestamp.Unix()), metric.Value},
			},
			Type: typeGauge,
			Host: metric.Host,
			Tags: metric.Tags,
		},
		)
	}
	return series
}

type Counter map[string]*Metric

func (m *Counter) Count(chanSeries chan Series, newMetrics Counter) {
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

func (m *Counter) GetSeries(newMetrics Counter) []Series {
	var series []Series
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
		series = append(series, Series{
			Metric: newMetric.Name,
			Points: [][]float64{
				{float64(prevMetric.Timestamp.Unix()), metricsValue},
			},
			Type:     typeCount,
			Interval: math.Round(newMetric.Timestamp.Sub(prevMetric.Timestamp).Seconds()),
			Host:     newMetric.Host,
			Tags:     newMetric.Tags,
		},
		)
	}
	return series
}
