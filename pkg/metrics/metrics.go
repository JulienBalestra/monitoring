package metrics

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/fnv"
)

const (
	TypeCount        = "count"
	TypeGauge        = "gauge"
	TypeDefaultGauge = ""

	DefaultMeasureMaxAgeSample = time.Hour * 12
)

var (
	errCountZero     = errors.New("count value is zero")
	errCountNegative = errors.New("count value is negative")
)

type Series struct {
	Metric   string      `json:"metric"`
	Points   [][]float64 `json:"points"`
	Type     string      `json:"type,omitempty"`
	Interval float64     `json:"interval,omitempty"`
	Host     string      `json:"host"`
	Tags     []string    `json:"tags,omitempty"`
}

type Sample struct {
	Name  string
	Value float64
	Time  time.Time
	Host  string

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
	interval := newMetric.Time.Sub(s.Time).Seconds()
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval for %q <-> %q : %.2f", s, newMetric, interval)
	}
	metricsValue := newMetric.Value - s.Value
	if metricsValue == 0 {
		return nil, errCountZero
	}
	// the counter might has been reset
	if metricsValue < 0 {
		return nil, errCountNegative
	}
	return &Series{
		Metric: newMetric.Name,
		Points: [][]float64{
			{float64(newMetric.Time.Unix()), metricsValue},
		},
		Type: TypeCount,
		// Datadog resolution is at the second
		Interval: math.Round(interval),
		Host:     newMetric.Host,
		Tags:     newMetric.Tags,
	}, nil
}

func (s *Sample) String() string {
	return fmt.Sprintf("%s %.2f %s %s %s %d", s.Name, s.Value, s.Time.Format(time.RFC3339), s.Host, s.Tags, s.Hash())
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
	return NewMeasuresWithMaxAge(ch, DefaultMeasureMaxAgeSample)
}

func NewMeasuresWithMaxAge(ch chan Series, maxAge time.Duration) *Measures {
	return &Measures{
		counter:   make(map[uint64]*Sample),
		deviation: make(map[uint64]*Sample),
		ch:        ch,
		purge:     time.Now(),
		maxAge:    maxAge,
	}
}

func (m *Measures) Purge() (float64, float64) {
	counts := 0.
	deviations := 0.
	if time.Since(m.purge) < m.maxAge {
		return counts, deviations
	}
	for key, sample := range m.counter {
		if time.Since(sample.Time) > m.maxAge {
			delete(m.counter, key)
			counts++
		}
	}
	for key, sample := range m.deviation {
		if time.Since(sample.Time) > m.maxAge {
			delete(m.deviation, key)
			deviations++
		}
	}
	m.purge = time.Now()
	return counts, deviations
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
			{float64(newSample.Time.Unix()), newSample.Value},
		},
		Type: TypeDefaultGauge,
		Host: newSample.Host,
		Tags: newSample.Tags,
	}
}

func (m *Measures) GaugeDeviation(newSample *Sample, maxAge time.Duration) bool {
	h := newSample.Hash()
	oldSample, ok := m.deviation[h]
	if ok && newSample.Value == oldSample.Value && time.Since(oldSample.Time) < maxAge {
		return false
	}
	m.deviation[h] = newSample
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
		Name:  newSample.Name,
		Value: newSample.Value + oldSample.Value,
		Time:  newSample.Time,
		Host:  newSample.Host,
		Tags:  newSample.Tags, // keep the same underlying array
	})
	if err != nil {
		if err != errCountZero {
			return err
		}
		m.counter[h] = newSample
		return nil
	}
	m.counter[h] = newSample
	m.ch <- *s
	return nil
}

func (m *Measures) Count(newSample *Sample) error {
	return m.count(newSample, false)
}

func (m *Measures) CountWithNegativeReset(newSample *Sample) error {
	return m.count(newSample, true)
}

func (m *Measures) count(newSample *Sample, resetNegative bool) error {
	h := newSample.Hash()
	oldSample, ok := m.counter[h]
	if !ok {
		m.counter[h] = newSample
		return nil
	}
	s, err := oldSample.Count(newSample)
	if err == nil {
		m.counter[h] = newSample
		m.ch <- *s
		return nil
	}
	if IsCountZero(err) {
		m.counter[h] = newSample
		return nil
	}
	if !resetNegative {
		return err
	}
	if !IsCountNegative(err) {
		return err
	}
	m.counter[h] = newSample
	return nil
}

func IsCountZero(err error) bool {
	return err == errCountZero
}

func IsCountNegative(err error) bool {
	return err == errCountNegative
}
