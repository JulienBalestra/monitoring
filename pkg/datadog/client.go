package datadog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/metrics"
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
	Host     string
	ChanSize int

	DatadogAPIKey string
	DatadogAPPKey string

	SendInterval  time.Duration
	ClientMetrics *ClientMetrics
}

type ClientMetrics struct {
	sync.RWMutex

	SentBytes  float64
	SentSeries float64
	SentErrors float64

	StoreAggregations float64
}

type Client struct {
	conf *Config

	httpClient             *http.Client
	seriesURL, hostTagsURL string

	ChanSeries    chan metrics.Series
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
	if conf.SendInterval == 0 {
		conf.SendInterval = time.Minute
	}
	return &Client{
		httpClient: httpClient,
		conf:       conf,

		seriesURL:   "https://api.datadoghq.com/api/v1/series?api_key=" + conf.DatadogAPIKey,
		hostTagsURL: "https://api.datadoghq.com/api/v1/tags/hosts/" + conf.Host,
		ChanSeries:  make(chan metrics.Series, conf.ChanSize),

		ClientMetrics: clientMetrics,
	}
}

type Payload struct {
	Series []metrics.Series `json:"series"`
}

type HostTags struct {
	Host string   `json:"host"`
	Tags []string `json:"tags"`
}

func (c *Client) UpdateHostTags(ctx context.Context, tags []string) error {
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(&HostTags{
		Host: c.conf.Host,
		Tags: tags,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.hostTagsURL, &buff)
	if err != nil {
		return err
	}
	req.Header.Set(contentType, applicationJson)
	req.Header.Set("DD-API-KEY", c.conf.DatadogAPIKey)
	req.Header.Set("DD-APPLICATION-KEY", c.conf.DatadogAPPKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 300 {
		apiKey, err := hideKey(c.conf.DatadogAPIKey)
		if err != nil {
			return fmt.Errorf("failed to update host tags status code: %d: %v", resp.StatusCode, err)
		}
		appKey, err := hideKey(c.conf.DatadogAPPKey)
		if err != nil {
			return fmt.Errorf("failed to update host tags status code: %d: %v", resp.StatusCode, err)
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to update host tags status code: %d %s: %v", resp.StatusCode, string(bodyBytes), err)
		}
		return fmt.Errorf("failed to update host tags status code: %d APP=%q API=%q %s", resp.StatusCode, appKey, apiKey, string(bodyBytes))
	}
	return nil
}

func (c *Client) Run(ctx context.Context) {
	// TODO explain these magic numbers
	const shutdownTimeout = 5 * time.Second
	failures, failuresDropThreshold := 0, 300/int(c.conf.SendInterval.Seconds())

	store := metrics.NewAggregateStore()

	ticker := time.NewTicker(c.conf.SendInterval)
	defer ticker.Stop()
	log.Printf("sending metrics every %s", c.conf.SendInterval)

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
			aggregateCount := store.Aggregate(&s)
			c.ClientMetrics.Lock()
			c.ClientMetrics.StoreAggregations += float64(aggregateCount)
			c.ClientMetrics.Unlock()

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

func hideKey(key string) (string, error) {
	const end = "***"
	if key == "" {
		return "", errors.New("invalid empty API/APP Key")
	}
	if len(key) < 8 {
		return "", errors.New("invalid API/APP Key")
	}
	return key[:8] + end, nil
}

func (c *Client) SendSeries(ctx context.Context, series []metrics.Series) error {
	b, err := json.Marshal(Payload{Series: series})
	if err != nil {
		return err
	}

	// TODO find a good logger/workflow to debug this
	//log.Printf("%s", string(b))
	//return nil

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.seriesURL, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set(contentType, applicationJson)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 300 {
		k, err := hideKey(c.conf.DatadogAPIKey)
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
	c.ChanSeries <- metrics.Series{
		Metric: "client.up",
		Points: [][]float64{
			{
				float64(time.Now().Unix()),
				1.0,
			},
		},
		Type: metrics.TypeGauge,
		Host: host,
		Tags: tags,
	}
}

func (c *Client) MetricClientShutdown(ctx context.Context, host string, tags ...string) error {
	return c.SendSeries(ctx, []metrics.Series{
		{
			Metric: "client.shutdown",
			Points: [][]float64{
				{
					float64(time.Now().Unix()),
					1.0,
				},
			},
			Type: metrics.TypeGauge,
			Host: host,
			Tags: tags,
		},
	})
}
