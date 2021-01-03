package metrics

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	host       = "host"
	metricName = "custom.metric"
	tag1       = "role:web"
	tag2       = "tier:db"
)

func TestNewAggregateStore(t *testing.T) {
	ts1 := float64(time.Now().Unix())
	ts2 := float64(time.Now().Unix() + 1)
	old := float64(time.Now().Add(-time.Hour * 2).Unix())
	for n, tc := range map[string]struct {
		series         []*Series
		expectedSeries []Series
	}{
		"nothing": {
			[]*Series{},
			[]Series{},
		},
		"same": {
			[]*Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
			[]Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
		},
		"one aggregation": {
			[]*Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts2,
							5,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
			[]Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
						{
							ts2,
							5,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
		},
		"no aggregation missing tag": {
			[]*Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag2},
				},
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts2,
							5,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
			[]Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag2},
				},
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts2,
							5,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
		},
		"garbage collection of one point": {
			[]*Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
						{
							ts2,
							4,
						},
						{
							old,
							6,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag2},
				},
			},
			[]Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							ts1,
							2,
						},
						{
							ts2,
							4,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag2},
				},
			},
		},
		"garbage collection of all points": {
			[]*Series{
				{
					Metric: metricName,
					Points: [][]float64{
						{
							old,
							6,
						},
					},
					Type: TypeDefaultGauge,
					Host: host,
					Tags: []string{tag2},
				},
			},
			[]Series{},
		},
	} {
		t.Run(n, func(t *testing.T) {
			s := NewAggregationStore()
			l := 0
			for _, se := range tc.series {
				l += s.Aggregate(se)
			}
			s.GarbageCollect(DatadogMetricsMaxAge())
			r := s.Series()
			sort.Slice(r, func(i, j int) bool {
				return len(r[i].Tags) < len(r[j].Tags)
			})
			assert.Equal(t, tc.expectedSeries, r)
		})
	}
}
