package datadog

import (
	"bytes"
	"compress/zlib"
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
	contentType         = "Content-Type"
	typeApplicationJson = "application/json"

	contentEncoding = "Content-Encoding"
	encodingDeflate = "deflate"

	MinimalSendInterval = time.Second * 5
	DefaultSendInterval = time.Second * 60
)

type Config struct {
	Host     string
	HostTags []string
	ChanSize int

	DatadogAPIKey string
	DatadogAPPKey string

	SendInterval  time.Duration
	ClientMetrics *ClientMetrics
	Logger        *zap.Config
}

type ClientMetrics struct {
	sync.RWMutex

	SentLogsBytes  float64
	SentLogsErrors float64

	SentSeriesBytes  float64
	SentSeries       float64
	SentSeriesErrors float64

	StoreAggregations float64
}

type Client struct {
	conf *Config

	httpClient                      *http.Client
	seriesURL, hostTagsURL, logsURL string

	ChanSeries chan metrics.Series
	Stats      *ClientMetrics
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
	if conf.SendInterval <= MinimalSendInterval {
		conf.SendInterval = DefaultSendInterval
	}
	return &Client{
		httpClient: httpClient,
		conf:       conf,

		seriesURL:   "https://api.datadoghq.com/api/v1/series?api_key=" + conf.DatadogAPIKey,
		hostTagsURL: "https://api.datadoghq.com/api/v1/tags/hosts/" + conf.Host,
		logsURL: "https://http-intake.logs.datadoghq.com/v1/input/" + conf.DatadogAPIKey +
			"?hostname=" + conf.Host,
		ChanSeries: make(chan metrics.Series, conf.ChanSize),

		Stats: clientMetrics,
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
	if len(tags) == 0 {
		return nil
	}

	// TODO find a good logger/workflow to debug this
	zap.L().Debug("sending host tags", zap.Strings("tags", tags))

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
	req.Header.Set(contentType, typeApplicationJson)
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
		return fmt.Errorf("failed to update host tags status code: %d: %v %s %q", resp.StatusCode, err, string(bodyBytes), tags)
	}
	appKey, err := hideKey(c.conf.DatadogAPPKey)
	if err != nil {
		return fmt.Errorf("failed to update host tags status code: %d: %v %s %q", resp.StatusCode, err, string(bodyBytes), tags)
	}
	return fmt.Errorf("failed to update host tags status code: %d APP=%q API=%q %s %q", resp.StatusCode, appKey, apiKey, string(bodyBytes), tags)
}

func (c *Client) Run(ctx context.Context) {
	const timeout = 5 * time.Second

	store := metrics.NewAggregationStore()

	seriesTicker := time.NewTicker(c.conf.SendInterval)
	defer seriesTicker.Stop()

	// TODO fix this
	//hostTicker := time.NewTicker(time.Hour)
	//defer hostTicker.Stop()

	zap.L().Info("sending metrics periodically", zap.Duration("sendInterval", c.conf.SendInterval))

	for {
		select {
		/*
			case <-hostTicker.C:
				zctx := zap.L().With(
					zap.Strings("hostTags", c.conf.HostTags),
				)
				ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
				err := c.UpdateHostTags(ctxTimeout, c.conf.HostTags)
				cancel()
				if err != nil {
					zctx.Error("failed to update host tags", zap.Error(err))
					continue
				}
				zctx.Info("successfully updated host tags")
		*/

		case <-ctx.Done():
			storeLen := store.Len()
			if storeLen > 0 {
				zctx := zap.L().With(
					zap.Int("storeLen", storeLen),
					zap.Duration("timeout", timeout),
				)
				// TODO find something better
				zctx.Info("sending pending series")
				ctxTimeout, cancel := context.WithTimeout(context.TODO(), timeout)
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
			c.Stats.Lock()
			c.Stats.StoreAggregations += float64(aggregateCount)
			c.Stats.Unlock()

		case <-seriesTicker.C:
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
				store.Reset()
				continue
			}
			c.Stats.Lock()
			c.Stats.SentSeriesErrors++
			c.Stats.Unlock()
			gcThreshold := metrics.DatadogMetricsMaxAge()
			gc := store.GarbageCollect(gcThreshold)
			zctx.Error("failed to send series",
				zap.Error(err),
				zap.Int("garbageCollected", gc),
				zap.Float64("garbageCollectionThreshold", gcThreshold),
			)
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
	if len(series) == 0 {
		return nil
	}

	var zb bytes.Buffer
	w, err := zlib.NewWriterLevel(&zb, zlib.BestCompression)
	if err != nil {
		return err
	}
	err = json.NewEncoder(w).Encode(Payload{Series: series})
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	bodyLen := float64(zb.Len())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.seriesURL, &zb)
	if err != nil {
		return err
	}
	req.Header.Set(contentType, typeApplicationJson)
	req.Header.Set(contentEncoding, encodingDeflate)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 300 {
		// internal self metrics/counters
		c.Stats.Lock()
		c.Stats.SentSeriesBytes += bodyLen
		c.Stats.SentSeries += float64(len(series))
		c.Stats.Unlock()

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
		return fmt.Errorf("failed to send series status code: %d: %v %s", resp.StatusCode, err, string(bodyBytes))
	}
	return fmt.Errorf("failed to usend series status code: %d API=%q %s", resp.StatusCode, apiKey, string(bodyBytes))
}

func (c *Client) SendLogs(ctx context.Context, buffer *bytes.Buffer) error {
	bufferLen := buffer.Len()
	if bufferLen == 0 {
		return nil
	}

	logsBytes := float64(bufferLen)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.logsURL, buffer)
	if err != nil {
		return err
	}

	req.Header.Set(contentType, "text/plain")
	req.Header.Set(contentEncoding, encodingDeflate)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.Stats.Lock()
		c.Stats.SentLogsErrors++
		c.Stats.Unlock()
		return err
	}
	if resp.StatusCode < 300 {
		// internal self metrics/counters
		c.Stats.Lock()
		c.Stats.SentLogsBytes += logsBytes
		c.Stats.Unlock()

		// From https://golang.org/pkg/net/http/#Response:
		// The default HTTP client's Transport may not reuse HTTP/1.x "keep-alive"
		// TCP connections if the Body is not read to completion and closed.
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		return resp.Body.Close()
	}
	c.Stats.Lock()
	c.Stats.SentLogsErrors++
	c.Stats.Unlock()

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
		Type:   metrics.TypeGauge,
		Points: [][]float64{
			{
				float64(time.Now().Unix()),
				1,
			},
		},
		Host: host,
		Tags: tags,
	}
}

func (c *Client) MetricClientShutdown(ctx context.Context, host string, tags ...string) error {
	return c.SendSeries(ctx, []metrics.Series{
		{
			Metric: "client.shutdown",
			Type:   metrics.TypeGauge,
			Points: [][]float64{
				{
					float64(time.Now().Unix()),
					1,
				},
			},
			Host: host,
			Tags: tags,
		},
	})
}
