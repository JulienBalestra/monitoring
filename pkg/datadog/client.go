package datadog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
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
)

type Config struct {
	Host string

	DatadogAPIKey string

	SendInterval  time.Duration
	ClientMetrics *ClientMetrics
}

type ClientMetrics struct {
	sync.RWMutex

	SentBytes  float64
	SentSeries float64
	SentErrors float64
}

type Client struct {
	conf *Config

	httpClient *http.Client
	url        string

	ChanSeries    chan Series
	ClientMetrics *ClientMetrics
}

func NewClient(conf *Config) *Client {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // TODO this is needed on dd-wrt, load certificate authority there
			},
		},
		Timeout: time.Second * 15,
	}
	clientMetrics := conf.ClientMetrics
	if conf.ClientMetrics == nil {
		clientMetrics = &ClientMetrics{}
	}
	return &Client{
		httpClient: httpClient,
		conf:       conf,

		url:        "https://api.datadoghq.com/api/v1/series?api_key=" + conf.DatadogAPIKey,
		ChanSeries: make(chan Series),

		ClientMetrics: clientMetrics,
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
	failures, failuresDropThreshold := 0, 300/int(c.conf.SendInterval.Seconds())

	store := NewAggregateStore()

	ticker := time.NewTicker(c.conf.SendInterval)
	defer ticker.Stop()
	log.Printf("starting datadog client")

	for {
		select {
		case <-ctx.Done():
			if store.Len() > 0 {
				// TODO find something better
				log.Printf("sending %d pending series with %s timeout", store.Len(), shutdownTimeout)
				ctxTimeout, cancel := context.WithTimeout(context.TODO(), shutdownTimeout)
				err := c.SendSeries(ctxTimeout, store.Series())
				cancel()
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
			ctxTimeout, cancel := context.WithTimeout(ctx, c.conf.SendInterval)
			err := c.SendSeries(ctxTimeout, store.Series())
			cancel()
			if err == nil {
				log.Printf("successfully sent %d series", store.Len())
				failures = 0
				store.Reset()
				continue
			}

			c.ClientMetrics.Lock()
			c.ClientMetrics.SentErrors++
			c.ClientMetrics.Unlock()

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

func (c *Client) hideAPIKey() (string, error) {
	const end = "***"
	if c.conf.DatadogAPIKey == "" {
		return "", errors.New("invalid empty API Key")
	}
	if len(c.conf.DatadogAPIKey) < 8 {
		return "", errors.New("invalid API Key")
	}
	return c.conf.DatadogAPIKey[:8] + end, nil
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
		k, err := c.hideAPIKey()
		if err != nil {
			return fmt.Errorf("failed to send series status code: %d: %v", resp.StatusCode, err)
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to send series status code: %d %s: %v", resp.StatusCode, string(bodyBytes), err)
		}
		return fmt.Errorf("failed to send series status code: %d api_key=%q %s", resp.StatusCode, k, string(bodyBytes))
	}

	// internal self metrics/counters
	c.ClientMetrics.Lock()
	c.ClientMetrics.SentBytes += float64(len(b))
	c.ClientMetrics.SentSeries += float64(len(series))
	c.ClientMetrics.Unlock()
	return nil
}

func (c *Client) MetricClientUp(host string, tags ...string) {
	c.ChanSeries <- Series{
		Metric: "client.up",
		Points: [][]float64{
			{
				float64(time.Now().Unix()),
				1.0,
			},
		},
		Type: TypeGauge,
		Host: host,
		Tags: tags,
	}
}

func (c *Client) MetricClientShutdown(ctx context.Context, host string, tags ...string) error {
	return c.SendSeries(ctx, []Series{
		{
			Metric: "client.shutdown",
			Points: [][]float64{
				{
					float64(time.Now().Unix()),
					1.0,
				},
			},
			Type: TypeGauge,
			Host: host,
			Tags: tags,
		},
	})
}
