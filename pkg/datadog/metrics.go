package datadog

import (
	"fmt"
	"math"
	"time"
)

const (
	TypeCount = "count"
	TypeGauge = "gauge"
)

type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Host      string

	Tags []string
}

func (m *Metric) Copy() *Metric {
	tags := make([]string, 0, len(m.Tags))
	copy(tags, m.Tags)
	return &Metric{
		Name:      m.Name,
		Value:     m.Value,
		Timestamp: m.Timestamp,
		Host:      m.Host,
		Tags:      tags,
	}
}

type Gauge []*Metric

func (m *Gauge) Gauge(chanSeries chan Series) {
	for _, metric := range *m {
		chanSeries <- Series{
			Metric: metric.Name,
			Points: [][]float64{
				{float64(metric.Timestamp.Unix()), metric.Value},
			},
			Type: TypeGauge,
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
			Type: TypeGauge,
			Host: metric.Host,
			Tags: metric.Tags,
		},
		)
	}
	return series
}

type Counter map[string]*Metric

func (m *Metric) Count(newMetric *Metric) (*Series, error) {
	interval := newMetric.Timestamp.Sub(m.Timestamp).Seconds()
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval %f", interval)
	}
	metricsValue := newMetric.Value - m.Value
	// count must be > 0
	if metricsValue <= 0 {
		return nil, fmt.Errorf("invalid value %f", metricsValue)
	}
	return &Series{
		Metric: newMetric.Name,
		Points: [][]float64{
			{float64(newMetric.Timestamp.Unix()), metricsValue},
		},
		Type:     TypeCount,
		Interval: interval,
		Host:     newMetric.Host,
		Tags:     newMetric.Tags,
	}, nil
}

// Count Count and adds to newCounters any missing metric previously registered
func (c *Counter) Count(chanSeries chan Series, newCounter Counter) Counter {
	for path, prevMetric := range *c {
		newMetric, ok := newCounter[path]
		if !ok {
			newCounter[path] = prevMetric
			continue
		}
		s, err := prevMetric.Count(newMetric)
		if err == nil {
			chanSeries <- *s
		}
	}
	return newCounter
}

func (c *Counter) Copy() Counter {
	newCounter := make(Counter, len(*c))
	for k, m := range *c {
		newCounter[k] = m.Copy()
	}
	return newCounter
}

func (c *Counter) GetSeries(newMetrics Counter) []Series {
	var series []Series
	for path, prevMetric := range *c {
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
			Type:     TypeCount,
			Interval: math.Round(newMetric.Timestamp.Sub(prevMetric.Timestamp).Seconds()),
			Host:     newMetric.Host,
			Tags:     newMetric.Tags,
		},
		)
	}
	return series
}
