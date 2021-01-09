package freebox

import (
	"context"
	"net/http"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/http_collector"
)

const (
	CollectorName = "freebox"

	OptionURL    = "url"
	optionMethod = "method"
)

type Collector struct {
	conf *collector.Config

	httpCollector collector.Collector
}

func NewFreebox(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:          conf,
		httpCollector: http_collector.NewHTTP(conf),
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
		OptionURL:    "http://mafreebox.freebox.fr/api_version",
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
	return c.httpCollector.Collect(ctx)
}
