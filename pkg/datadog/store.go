package datadog

import (
	"hash/fnv"
	"strconv"
)

type AggregateStore struct {
	store map[uint64]*Series
}

func NewAggregateStore() *AggregateStore {
	return &AggregateStore{store: make(map[uint64]*Series)}
}

func (st *AggregateStore) Reset() {
	st.store = make(map[uint64]*Series)
}

func (st *AggregateStore) Series() []Series {
	series := make([]Series, 0)
	for _, s := range st.store {
		series = append(series, *s)
	}
	return series
}

func (st *AggregateStore) Aggregate(series ...*Series) {
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
			return
		}
		existing.Points = append(existing.Points, s.Points...)
	}
}

func (st *AggregateStore) Len() int {
	return len(st.store)
}
