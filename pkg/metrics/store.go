package metrics

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/fnv"
)

const maxSerieAge = time.Minute * 59

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

// Reset with 90% of the previous size
func (st *AggregationStore) Reset() {
	st.mu.Lock()
	st.store = make(map[uint64]*Series, int(math.Round(float64(len(st.store))*0.9)))
	st.mu.Unlock()
}

func (st *AggregationStore) GarbageCollect() int {
	threshold := float64(time.Now().Add(-time.Hour).Unix())
	gc := 0
	st.mu.Lock()
	for k, v := range st.store {
		i := 0
		for _, p := range v.Points {
			if p[0] < threshold {
				gc++
				continue
			}
			v.Points[i] = p
			i++
		}
		for j := i; j < len(v.Points); j++ {
			v.Points[j][0] = 0
			v.Points[j][1] = 0
			v.Points[j] = nil
		}
		if i == 0 {
			delete(st.store, k)
			continue
		}
		v.Points = v.Points[:i]
	}
	st.mu.Unlock()
	return gc
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
