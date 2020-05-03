package metrics

import (
	"fmt"
	"hash/fnv"
	"sort"
	"time"
)

const (
	TypeCount = "count"
	TypeGauge = "gauge"
)

type Series struct {
	Metric   string      `json:"metric"`
	Points   [][]float64 `json:"points"`
	Type     string      `json:"type"`
	Interval float64     `json:"interval,omitempty"`
	Host     string      `json:"host"`
	Tags     []string    `json:"tags"`
}

type Sample struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Host      string

	Tags []string
}

type Measures struct {
	counter   map[uint64]*Sample
	deviation map[uint64]*Sample
	ch        chan Series
}

func (s *Sample) Count(newMetric *Sample) (*Series, error) {
	interval := newMetric.Timestamp.Sub(s.Timestamp).Seconds()
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval for %q <-> %q : %.2f", s, newMetric, interval)
	}
	metricsValue := newMetric.Value - s.Value
	// count must be > 0
	if metricsValue <= 0 {
		return nil, fmt.Errorf("invalid value for %q <-> %q : %.2f", s, newMetric, metricsValue)
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

func (s *Sample) String() string {
	return fmt.Sprintf("%s %.2f %s %s %s %d", s.Name, s.Value, s.Timestamp.Format(time.RFC3339), s.Host, s.Tags, s.Hash())
}

func (s *Sample) Hash() uint64 {
	h := fnv.New64()
	_, _ = h.Write([]byte(s.Name))
	_, _ = h.Write([]byte(s.Host))
	sort.Strings(s.Tags)
	for _, tag := range s.Tags {
		_, _ = h.Write([]byte(tag))
	}
	return h.Sum64()
}

func NewMeasures(ch chan Series) *Measures {
	return &Measures{
		counter:   make(map[uint64]*Sample),
		deviation: make(map[uint64]*Sample),
		ch:        ch,
	}
}

func (m *Measures) Gauge(newSample *Sample) {
	m.ch <- Series{
		Metric: newSample.Name,
		Points: [][]float64{
			{float64(newSample.Timestamp.Unix()), newSample.Value},
		},
		Type: TypeGauge,
		Host: newSample.Host,
		Tags: newSample.Tags,
	}
}

func (m *Measures) GaugeDeviation(newSample *Sample, maxAge time.Duration) bool {
	h := newSample.Hash()
	oldSample, ok := m.counter[h]
	if ok && newSample.Value == oldSample.Value && time.Since(oldSample.Timestamp) < maxAge {
		return false
	}
	m.counter[h] = newSample
	m.Gauge(newSample)
	return true
}

func (m *Measures) Incr(newSample *Sample) error {
	h := newSample.Hash()
	oldSample, ok := m.counter[h]
	if !ok {
		m.counter[h] = newSample
		return nil
	}
	s, err := oldSample.Count(&Sample{
		Name:      newSample.Name,
		Value:     newSample.Value + oldSample.Value,
		Timestamp: newSample.Timestamp,
		Host:      newSample.Host,
		Tags:      newSample.Tags, // keep the same underlying array
	})
	if err != nil {
		return err
	}
	m.counter[h] = newSample
	m.ch <- *s
	return nil
}

func (m *Measures) Count(newSample *Sample) error {
	h := newSample.Hash()
	oldSample, ok := m.counter[h]
	if !ok {
		m.counter[h] = newSample
		return nil
	}
	s, err := oldSample.Count(newSample)
	if err != nil {
		return err
	}
	m.counter[h] = newSample
	m.ch <- *s
	return nil
}
