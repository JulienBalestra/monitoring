package http_collector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
	optionMethod = "method"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
	client   *http.Client
}

func NewHTTP(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
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
		optionMethod: http.MethodGet,
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
	m, ok := c.conf.Options[optionMethod]
	if !ok {
		m = http.MethodGet
		zap.L().Debug("missing option",
			zap.String("options", optionMethod),
			zap.String(optionMethod, http.MethodGet),
		)
	}
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, m, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	tags := append(c.conf.Tagger.GetUnstable(u.Host), c.Tags()...)
	tags = append(tags,
		"code:"+strconv.Itoa(resp.StatusCode),
		"url:"+s,
		"host:"+u.Host,
		"collector:"+CollectorName,
		"method:"+m,
	)
	if net.ParseIP(u.Host) != nil {
		tags = append(tags, "ip:"+u.Host)
	}
	port := u.Port()
	if strings.HasPrefix(s, "https://") {
		tags = append(tags, "scheme:"+"https")
		if port == "" {
			port = "443"
		}
	} else {
		tags = append(tags, "scheme:"+"http")
		if port == "" {
			port = "80"
		}
	}
	tags = append(tags, "port:"+port)
	path := u.Path
	if path == "" {
		path = "/"
	}
	tags = append(tags, "path:"+path)
	_ = c.measures.Incr(
		&metrics.Sample{
			Name:  "http.query",
			Value: 1,
			Time:  time.Now(),
			Host:  c.conf.Host,
			Tags:  tags,
		},
	)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request for URL %s returned HTTP status %s", req.URL.String(), resp.Status)
	}
	return nil
}
