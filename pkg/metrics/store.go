package metrics

import (
	"hash/fnv"
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
		h := fnv.New64()
		_, _ = h.Write([]byte(s.Metric))
		_, _ = h.Write([]byte(s.Host))
		_, _ = h.Write([]byte(s.Type))
		_, _ = h.Write([]byte(strconv.FormatInt(int64(s.Interval), 10)))

		for _, tag := range s.Tags {
			_, _ = h.Write([]byte(tag))
		}
		hash := h.Sum64()

		existing, ok := st.store[hash]
		if !ok {
			st.store[hash] = s
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
