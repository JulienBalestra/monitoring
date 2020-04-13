package datadog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	clientSentBytesMetric  = "client.sent.bytes"
	clientSentSeriesMetric = "client.sent.series"
)

type Client struct {
	tagger *Tagger

	httpClient *http.Client
	url        string
	host       string

	ChanSeries chan Series
}

func NewClient(host, apiKey string, tagger *Tagger) *Client {
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

// TODO aggregate same timeseries

func (c *Client) Run(ctx context.Context) {
	// TODO explain these magic numbers
	const tickerPeriod = 20 * time.Second
	failures, failuresDropThreshold := 0, 300/int(tickerPeriod.Seconds())
	series := make([]Series, 0, 1000)

	ticker := time.NewTicker(tickerPeriod)
	defer ticker.Stop()
	log.Printf("starting datadog client")

	var counters CounterMap
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of datadog client")
			return

		case s := <-c.ChanSeries:
			series = append(series, s)

		case <-ticker.C:
			if len(series) == 0 {
				log.Printf("no series cached")
				continue
			}
			ctxTimeout, _ := context.WithTimeout(ctx, tickerPeriod)
			newCounter, err := c.SendSeries(ctxTimeout, series)
			if err == nil {
				log.Printf("successfully sent %d series", len(series))
				failures = 0
				series = series[:0]
				if counters != nil {
					series = append(series, counters.GetCountSeries(newCounter)...)
				}
				counters = newCounter
				continue
			}
			failures++
			log.Printf("failed to send %d series: %v", len(series), err)
			// TODO maybe use a rate limited queue
			if failures < failuresDropThreshold {
				log.Printf("attempt %d/%d: will drop the series over threshold", failures, failuresDropThreshold)
				continue
			}
			log.Printf("dropping %d series", len(series))
			failures = 0
			series = series[:0]
		}
	}
}

func (c *Client) SendSeries(ctx context.Context, series []Series) (CounterMap, error) {
	b, err := json.Marshal(Payload{Series: series})
	if err != nil {
		return nil, err
	}

	// TODO find a good logger to debug this
	//log.Printf("%s", string(b))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set(contentType, applicationJson)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 300 {
		return nil, fmt.Errorf("failed to send series status code: %d", resp.StatusCode)
	}

	now := time.Now()
	hostTags := c.tagger.Get(c.host)
	return CounterMap{
			clientSentBytesMetric: &Metric{
				Name:      clientSentBytesMetric,
				Value:     float64(len(b)),
				Host:      c.host,
				Timestamp: now,
				Tags:      hostTags,
			},
			clientSentSeriesMetric: &Metric{
				Name:      clientSentSeriesMetric,
				Value:     float64(len(series)),
				Host:      c.host,
				Timestamp: now,
				Tags:      hostTags,
			},
		},
		nil
}
