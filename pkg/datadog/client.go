package datadog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"hash/fnv"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

/*
curl  -X POST -H "Content-type: application/json" \
-d "{ \"series\" :
         [{\"metric\":\"test.metric\",
          \"points\":[[$currenttime, 20]],
          \"type\":\"rate\",
          \"interval\": 20,
          \"host\":\"test.example.com\",
          \"tags\":[\"environment:test\"]}
        ]
}" \
"https://api.datadoghq.com/api/v1/series?api_key="<DATADOG_API_KEY>""
*/

const (
	contentType     = "Content-Type"
	applicationJson = "application/json"

	clientSentByteMetric   = "client.sent.byte"
	clientSentSeriesMetric = "client.sent.series"
)

type Client struct {
	tagger *tagger.Tagger

	httpClient *http.Client
	url        string
	host       string

	ChanSeries chan Series

	// internal self metrics
	mu         sync.RWMutex
	sentBytes  float64
	sentSeries float64
}

func NewClient(host, apiKey string, tagger *tagger.Tagger) *Client {
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // TODO this is needed on dd-wrt, load certificate authority there
			},
		},
		Timeout: time.Second * 15,
	}
	return &Client{
		httpClient: c,
		url:        "https://api.datadoghq.com/api/v1/series?api_key=" + apiKey,
		host:       host,
		ChanSeries: make(chan Series),
		tagger:     tagger,
	}
}

type Payload struct {
	Series []Series `json:"series"`
}

type Series struct {
	Metric   string      `json:"metric"`
	Points   [][]float64 `json:"points"`
	Type     string      `json:"type"`
	Interval float64     `json:"interval,omitempty"`
	Host     string      `json:"host"`
	Tags     []string    `json:"tags"`
}

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
	var series []Series
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

func (c *Client) Run(ctx context.Context) {
	// TODO explain these magic numbers
	const shutdownTimeout = 5 * time.Second
	const tickerPeriod = 20 * time.Second
	failures, failuresDropThreshold := 0, 300/int(tickerPeriod.Seconds())

	store := NewAggregateStore()

	ticker := time.NewTicker(tickerPeriod)
	defer ticker.Stop()
	log.Printf("starting datadog client")

	var counters CounterMap
	for {
		select {
		case <-ctx.Done():
			if store.Len() > 0 {
				// TODO find something better
				log.Printf("sending %d pending series with %s timeout", store.Len(), shutdownTimeout)
				ctxTimeout, _ := context.WithTimeout(context.TODO(), shutdownTimeout)
				err := c.SendSeries(ctxTimeout, store.Series())
				if err != nil {
					log.Printf("still %d pending series: %v", store.Len(), err)
				}
			}
			log.Printf("end of datadog client")
			return

		case s := <-c.ChanSeries:
			store.Aggregate(&s)

		case <-ticker.C:
			if store.Len() == 0 {
				log.Printf("no series cached")
				continue
			}
			ctxTimeout, _ := context.WithTimeout(ctx, tickerPeriod)
			err := c.SendSeries(ctxTimeout, store.Series())
			if err == nil {
				log.Printf("successfully sent %d series", store.Len())
				failures = 0
				store.Reset()
				newCounter := c.GetClientCounter()
				if counters != nil {
					for _, s := range counters.GetCountSeries(newCounter) {
						store.Aggregate(&s)
					}
				}
				counters = newCounter
				continue
			}
			failures++
			log.Printf("failed to send %d series: %v", store.Len(), err)
			// TODO maybe use a rate limited queue
			if failures < failuresDropThreshold {
				log.Printf("attempt %d/%d: will drop the series over threshold", failures, failuresDropThreshold)
				continue
			}
			log.Printf("dropping %d series", store.Len())
			failures = 0
			store.Reset()
		}
	}
}

func (c *Client) GetClientCounter() CounterMap {
	now := time.Now()
	hostTags := c.tagger.Get(c.host)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CounterMap{
		clientSentByteMetric: &Metric{
			Name:      clientSentByteMetric,
			Value:     c.sentBytes,
			Host:      c.host,
			Timestamp: now,
			Tags:      hostTags,
		},
		clientSentSeriesMetric: &Metric{
			Name:      clientSentSeriesMetric,
			Value:     c.sentSeries,
			Host:      c.host,
			Timestamp: now,
			Tags:      hostTags,
		},
	}
}

func (c *Client) SendSeries(ctx context.Context, series []Series) error {
	b, err := json.Marshal(Payload{Series: series})
	if err != nil {
		return err
	}

	// TODO find a good logger to debug this
	//log.Printf("%s", string(b))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set(contentType, applicationJson)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 300 {
		return fmt.Errorf("failed to send series status code: %d", resp.StatusCode)
	}

	// internal self metrics/counters
	c.mu.Lock()
	c.sentBytes += float64(len(b))
	c.sentSeries += float64(len(series))
	c.mu.Unlock()
	return nil
}

func (c *Client) MetricClientUp(tags ...string) {
	c.ChanSeries <- Series{
		Metric: "client.up",
		Points: [][]float64{
			{
				float64(time.Now().Unix()),
				1.0,
			},
		},
		Type: typeGauge,
		Host: c.host,
		Tags: append(c.tagger.Get(c.host), tags...),
	}
}

func (c *Client) MetricClientShutdown(ctx context.Context, tags ...string) error {
	return c.SendSeries(ctx, []Series{
		{
			Metric: "client.shutdown",
			Points: [][]float64{
				{
					float64(time.Now().Unix()),
					1.0,
				},
			},
			Type: typeGauge,
			Host: c.host,
			Tags: append(c.tagger.Get(c.host), tags...),
		},
	})
}
