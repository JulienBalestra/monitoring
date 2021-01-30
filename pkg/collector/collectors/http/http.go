package http_collector

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "http"

	OptionURL    = "url"
	OptionMethod = "method"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
	client   *http.Client
}

func NewHTTP(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	})
}

func (c *Collector) SubmittedSeries() float64 {
	return c.measures.GetTotalSubmittedSeries()
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{
		OptionMethod: http.MethodGet,
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(ctx context.Context) error {
	s, ok := c.conf.Options[OptionURL]
	if !ok {
		zap.L().Error("missing option", zap.String("options", OptionURL))
		return errors.New("missing option " + OptionURL)
	}
	m, ok := c.conf.Options[OptionMethod]
	if !ok {
		m = http.MethodGet
		zap.L().Debug("defaulting option",
			zap.String("option", OptionMethod),
			zap.String(OptionMethod, http.MethodGet),
		)
	}
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, m, s, nil)
	if err != nil {
		return err
	}
	now := time.Now()
	resp, err := c.client.Do(req)
	latency := time.Since(now)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	tags := c.Tags()
	ipAddress := u.Host
	if net.ParseIP(ipAddress) == nil {
		ipAddress = "none"
	}
	port := u.Port()
	scheme := ""
	if strings.HasPrefix(s, "https") {
		scheme = "https"
		if port == "" {
			port = "443"
		}
	} else {
		scheme = "http"
		if port == "" {
			port = "80"
		}
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	_ = c.measures.GaugeDeviation(
		&metrics.Sample{
			Name:  "latency.http",
			Value: float64(latency.Milliseconds()),
			Time:  now,
			Host:  c.conf.Host,
			Tags: append(tags,
				"code:"+strconv.Itoa(resp.StatusCode),
				"url:"+s,
				"host-target:"+u.Host,
				"collector:"+CollectorName,
				"method:"+m,
				"path:"+path,
				"port:"+port,
				"ip:"+ipAddress,
				"scheme:"+scheme,
			),
		}, c.conf.CollectInterval,
	)
	return nil
}
