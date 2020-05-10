package datadog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	Logger        *zap.Config
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
	if conf.SendInterval <= time.Second*5 {
		conf.SendInterval = time.Second * 60
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

	if resp.StatusCode < 300 {
		// From https://golang.org/pkg/net/http/#Response:
		// The default HTTP client's Transport may not reuse HTTP/1.x "keep-alive"
		// TCP connections if the Body is not read to completion and closed.
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		return resp.Body.Close()
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	apiKey, err := hideKey(c.conf.DatadogAPIKey)
	if err != nil {
		return fmt.Errorf("failed to update host tags status code: %d: %v %s", resp.StatusCode, err, string(bodyBytes))
	}
	appKey, err := hideKey(c.conf.DatadogAPPKey)
	if err != nil {
		return fmt.Errorf("failed to update host tags status code: %d: %v %s", resp.StatusCode, err, string(bodyBytes))
	}
	return fmt.Errorf("failed to update host tags status code: %d APP=%q API=%q %s", resp.StatusCode, appKey, apiKey, string(bodyBytes))
}

func (c *Client) Run(ctx context.Context) {
	// TODO explain these magic numbers
	const shutdownTimeout = 5 * time.Second
	failures, failuresDropThreshold := 0, 300/int(c.conf.SendInterval.Seconds())

	store := metrics.NewAggregationStore()

	ticker := time.NewTicker(c.conf.SendInterval)
	defer ticker.Stop()
	zap.L().Info("sending metrics periodically", zap.Duration("sendInterval", c.conf.SendInterval))

	for {
		select {
		case <-ctx.Done():
			storeLen := store.Len()
			if storeLen > 0 {
				zctx := zap.L().With(
					zap.Int("storeLen", storeLen),
					zap.Duration("timeout", shutdownTimeout),
				)
				// TODO find something better
				zctx.Info("sending pending series")
				ctxTimeout, cancel := context.WithTimeout(context.TODO(), shutdownTimeout)
				err := c.SendSeries(ctxTimeout, store.Series())
				cancel()
				if err != nil {
					zctx.Error("end of datadog client with pending series", zap.Error(err))
					return
				}
			}
			zap.L().Info("end of datadog client")
			return

		case s := <-c.ChanSeries:
			aggregateCount := store.Aggregate(&s)
			c.ClientMetrics.Lock()
			c.ClientMetrics.StoreAggregations += float64(aggregateCount)
			c.ClientMetrics.Unlock()

		case <-ticker.C:
			storeLen := store.Len()
			zctx := zap.L().With(
				zap.Int("storeLen", storeLen),
			)
			if storeLen == 0 {
				zctx.Debug("no series cached")
				continue
			}
			ctxTimeout, cancel := context.WithTimeout(ctx, c.conf.SendInterval)
			err := c.SendSeries(ctxTimeout, store.Series())
			cancel()
			if err == nil {
				zctx.Info("successfully sent series")
				failures = 0
				store.Reset()
				continue
			}

			c.ClientMetrics.Lock()
			c.ClientMetrics.SentErrors++
			c.ClientMetrics.Unlock()

			failures++
			zctx = zctx.With(
				zap.Error(err),
				zap.Int("failures", failures),
				zap.Int("threshold", failuresDropThreshold),
			)
			// TODO maybe use a rate limited queue
			if failures < failuresDropThreshold {
				zctx.Warn("will drop series over threshold")
				continue
			}
			zctx.Error("dropping series")
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
	zap.L().Debug("sending series", zap.Any("series", series))
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
	if resp.StatusCode < 300 {
		// internal self metrics/counters
		c.ClientMetrics.Lock()
		c.ClientMetrics.SentBytes += float64(len(b))
		c.ClientMetrics.SentSeries += float64(len(series))
		c.ClientMetrics.Unlock()

		// From https://golang.org/pkg/net/http/#Response:
		// The default HTTP client's Transport may not reuse HTTP/1.x "keep-alive"
		// TCP connections if the Body is not read to completion and closed.
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		return resp.Body.Close()
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	apiKey, err := hideKey(c.conf.DatadogAPIKey)
	if err != nil {
		return fmt.Errorf("failed to update host tags status code: %d: %v %s", resp.StatusCode, err, string(bodyBytes))
	}
	return fmt.Errorf("failed to update host tags status code: %d API=%q %s", resp.StatusCode, apiKey, string(bodyBytes))
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
