package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricCount(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		prevMetric *Sample
		newMetric  *Sample
		exp        *Series
		err        bool
	}{
		"1": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Sample{
				Name:      "metric",
				Value:     2,
				Timestamp: now.Add(time.Second),
				Host:      "host",
				Tags:      []string{},
			},
			&Series{
				Metric: "metric",
				Points: [][]float64{
					{float64(now.Add(time.Second).Unix()), 1},
				},
				Type:     TypeCount,
				Interval: 1,
				Host:     "host",
				Tags:     []string{},
			},
			false,
		},
		"1:tags": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Sample{
				Name:      "metric",
				Value:     2,
				Timestamp: now.Add(time.Second),
				Host:      "host",
				Tags:      []string{"1:1"},
			},
			&Series{
				Metric: "metric",
				Points: [][]float64{
					{float64(now.Add(time.Second).Unix()), 1},
				},
				Type:     TypeCount,
				Interval: 1,
				Host:     "host",
				Tags:     []string{"1:1"},
			},
			false,
		},
		"0": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now.Add(time.Second),
				Host:      "host",
				Tags:      []string{"1:1"},
			},
			nil,
			true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, err := tc.prevMetric.Count(tc.newMetric)
			if tc.err {
				assert.Error(t, err)
				assert.Nil(t, s)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.exp, s)
			assert.Equal(t, tc.prevMetric.Hash(), tc.prevMetric.Hash())
		})
	}
}

func TestMetricHash(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		one *Sample
		two *Sample
	}{
		"1": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{"one", "two"},
			},
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{"two", "one"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.one.Hash(), tc.two.Hash())
		})
	}
}

func TestGaugeDeviation(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		sample    *Sample
		deviation bool
		maxAge    time.Duration
		len       int
	}{
		"true:0:2": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{"one", "two"},
			},
			true,
			0,
			2,
		},
		"false:0:1": {
			&Sample{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{"one", "two"},
			},
			false,
			time.Hour,
			1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ch := make(chan Series, 10)
			defer close(ch)
			m := NewMeasures(ch)
			assert.True(t, m.GaugeDeviation(tc.sample, tc.maxAge))
			assert.Equal(t, tc.deviation, m.GaugeDeviation(tc.sample, tc.maxAge))
			assert.Len(t, ch, tc.len, ch)
			// 0 will discard the deviation
			assert.True(t, m.GaugeDeviation(tc.sample, 0))
			assert.Len(t, ch, tc.len+1, ch)
			m.maxAge = 0
			m.Purge()
			assert.True(t, m.GaugeDeviation(tc.sample, time.Hour*24))
			assert.False(t, m.GaugeDeviation(tc.sample, time.Hour*24))
		})
	}
}

func BenchmarkSampleHash(b *testing.B) {
	s := &Sample{
		Name:      "metric",
		Value:     1,
		Timestamp: time.Now(),
		Host:      "host",
		Tags:      []string{"one", "two"},
	}
	s.Hash()
}
