package metrics

import (
	"strconv"
	"sync"
)

type AggregateStore struct {
	mu    *sync.RWMutex
	store map[uint64]*Series
}

func NewAggregateStore() *AggregateStore {
	return &AggregateStore{
		store: make(map[uint64]*Series),
		mu:    &sync.RWMutex{},
	}
}

// Reset with 75% of the previous size
func (st *AggregateStore) Reset() {
	st.mu.Lock()
	st.store = make(map[uint64]*Series, int(float64(len(st.store))*0.75))
	st.mu.Unlock()
}

func (st *AggregateStore) Series() []Series {
	st.mu.RLock()
	series := make([]Series, 0, len(st.store))
	for _, s := range st.store {
		series = append(series, *s)
	}
	st.mu.RUnlock()
	return series
}

func (st *AggregateStore) Aggregate(series ...*Series) int {
	matchingSeries := 0
	st.mu.Lock()
	for _, s := range series {
		h := hashNew()
		h = hashAdd(h, s.Metric)
		h = hashAdd(h, s.Host)
		h = hashAdd(h, s.Type)
		h = hashAdd(h, strconv.FormatInt(int64(s.Interval), 10))

		for _, tag := range s.Tags {
			h = hashAdd(h, tag)
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

func (st *AggregateStore) Len() int {
	st.mu.RLock()
	st.mu.RUnlock()
	return len(st.store)
}
