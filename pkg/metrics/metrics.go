package metrics

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/fnv"
)

const (
	TypeCount = "count"
	TypeGauge = "gauge"

	CountMaxAgeSample = time.Hour * 48
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

	purge  time.Time
	maxAge time.Duration
}

func (s *Sample) Count(newMetric *Sample) (*Series, error) {
	interval := newMetric.Timestamp.Sub(s.Timestamp).Seconds()
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval for %q <-> %q : %.2f", s, newMetric, interval)
	}
	metricsValue := newMetric.Value - s.Value
	// There is a logic error that should never happen
	if metricsValue < 0 {
		return nil, fmt.Errorf("invalid value %.2f for %q <-> %q", metricsValue, s, newMetric)
	}
	return &Series{
		Metric: newMetric.Name,
		Points: [][]float64{
			{float64(newMetric.Timestamp.Unix()), metricsValue},
		},
		Type: TypeCount,
		// Datadog resolution is at the second
		Interval: math.Round(interval),
		Host:     newMetric.Host,
		Tags:     newMetric.Tags,
	}, nil
}

func (s *Sample) String() string {
	return fmt.Sprintf("%s %.2f %s %s %s %d", s.Name, s.Value, s.Timestamp.Format(time.RFC3339), s.Host, s.Tags, s.Hash())
}

func (s *Sample) Hash() uint64 {
	h := fnv.NewHash()
	h = fnv.AddString(h, s.Name)
	h = fnv.AddString(h, s.Host)
	sort.Strings(s.Tags)
	for _, tag := range s.Tags {
		h = fnv.AddString(h, tag)
	}
	return h
}

func NewMeasures(ch chan Series) *Measures {
	return &Measures{
		counter:   make(map[uint64]*Sample),
		deviation: make(map[uint64]*Sample),
		ch:        ch,
		purge:     time.Now(),
		maxAge:    CountMaxAgeSample,
	}
}

func (m *Measures) Purge() {
	if time.Since(m.purge) < m.maxAge {
		return
	}
	for key, sample := range m.counter {
		if time.Since(sample.Timestamp) > m.maxAge {
			delete(m.counter, key)
		}
	}
	for key, sample := range m.deviation {
		if time.Since(sample.Timestamp) > m.maxAge {
			delete(m.counter, key)
		}
	}
	m.purge = time.Now()
}

func (m *Measures) Delete(sample *Sample) {
	h := sample.Hash()
	delete(m.deviation, h)
	delete(m.counter, h)
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
