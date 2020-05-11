package metrics

import (
	"strconv"
	"sync"

	"github.com/JulienBalestra/monitoring/pkg/fnv"
)

type AggregationStore struct {
	mu    *sync.RWMutex
	store map[uint64]*Series
}

func NewAggregationStore() *AggregationStore {
	return &AggregationStore{
		store: make(map[uint64]*Series),
		mu:    &sync.RWMutex{},
	}
}

// Reset with 75% of the previous size
func (st *AggregationStore) Reset() {
	st.mu.Lock()
	st.store = make(map[uint64]*Series, int(float64(len(st.store))*0.75))
	st.mu.Unlock()
}

func (st *AggregationStore) Series() []Series {
	st.mu.RLock()
	series := make([]Series, 0, len(st.store))
	for _, s := range st.store {
		series = append(series, *s)
	}
	st.mu.RUnlock()
	return series
}

func (st *AggregationStore) Aggregate(series ...*Series) int {
	matchingSeries := 0
	st.mu.Lock()
	for _, s := range series {
		h := fnv.NewHash()
		h = fnv.AddString(h, s.Metric)
		h = fnv.AddString(h, s.Host)
		h = fnv.AddString(h, s.Type)
		h = fnv.AddString(h, strconv.FormatInt(int64(s.Interval), 10))

		for _, tag := range s.Tags {
			h = fnv.AddString(h, tag)
		}
		existing, ok := st.store[h]
		if !ok {
			st.store[h] = s
			continue
		}
		matchingSeries++
		existing.Points = append(existing.Points, s.Points...)
	}
	st.mu.Unlock()
	return matchingSeries
}

func (st *AggregationStore) Len() int {
	st.mu.RLock()
	l := len(st.store)
	st.mu.RUnlock()
	return l
}
