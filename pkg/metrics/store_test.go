package metrics

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	host       = "host"
	metricName = "custom.metric"
	tag1       = "role:web"
	tag2       = "tier:db"

	ts1 = 1587310001
	ts2 = 1587310002
)

func TestNewAggregateStore(t *testing.T) {

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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
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
					Type: TypeGauge,
					Host: host,
					Tags: []string{tag1, tag2},
				},
			},
		},
	} {
		t.Run(n, func(t *testing.T) {
			s := NewAggregationStore()
			l := 0
			for _, se := range tc.series {
				l += s.Aggregate(se)
			}
			r := s.Series()
			sort.Slice(r, func(i, j int) bool {
				return len(r[i].Tags) < len(r[j].Tags)
			})
			assert.Equal(t, tc.expectedSeries, r)
			assert.Equal(t, len(tc.series)-len(tc.expectedSeries), l)
		})
	}
}
