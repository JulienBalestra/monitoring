package lunar

import (
	"context"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorLoadName = "lunar"
)

type Load struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewAcaia(conf *collector.Config) collector.Collector {
	return &Load{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
	}
}

func (c *Load) Config() *collector.Config {
	return c.conf
}

func (c *Load) IsDaemon() bool { return false }

func (c *Load) Name() string {
	return CollectorLoadName
}

func (c *Load) Collect(_ context.Context) error {

	return nil
}
