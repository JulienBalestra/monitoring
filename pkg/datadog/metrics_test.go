package datadog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricCount(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		prevMetric *Metric
		newMetric  *Metric
		exp        *Series
		err        bool
	}{
		"1": {
			&Metric{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Metric{
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
			&Metric{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Metric{
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
			&Metric{
				Name:      "metric",
				Value:     1,
				Timestamp: now,
				Host:      "host",
				Tags:      []string{},
			},
			&Metric{
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
		})
	}
}
