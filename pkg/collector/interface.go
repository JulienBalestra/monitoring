package collector

import (
	"context"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

type Config struct {
	MetricsClient *datadog.Client
	Tagger        *tagger.Tagger

	Host            string
	CollectInterval time.Duration
	Options         map[string]string
}

func (c Config) OverrideCollectInterval(d time.Duration) *Config {
	c.CollectInterval = d
	return &c
}

type Collector interface {
	Config() *Config
	Collect(context.Context) error
	Name() string
	IsDaemon() bool
	DefaultOptions() map[string]string
	DefaultCollectInterval() time.Duration
}

func RunCollection(ctx context.Context, c Collector) error {
	config := c.Config()
	measures := metrics.NewMeasures(config.MetricsClient.ChanSeries)

	zctx := zap.L().With(
		zap.String("collector", c.Name()),
		zap.Duration("collectionInterval", config.CollectInterval),
	)

	if c.IsDaemon() {
		zctx.Info("collecting metrics continuously")
		err := c.Collect(ctx)
		if err != nil {
			return err
		}
		return nil
	}

	ticker := time.NewTicker(config.CollectInterval)
	defer ticker.Stop()
	zctx.Info("collecting metrics periodically")
	collectorTag := "collector:" + c.Name()
	for {
		select {
		case <-ctx.Done():
			zctx.Info("end of collection")
			return ctx.Err()

		case <-ticker.C:
			s := &metrics.Sample{
				Name:      "collector.runs",
				Value:     1,
				Timestamp: time.Now(),
				Host:      config.Host,
				Tags:      append(config.Tagger.GetUnstable(config.Host), collectorTag),
			}
			err := c.Collect(ctx)
			if err != nil {
				zctx.Error("failed collection", zap.Error(err))
				s.Tags = append(s.Tags, "success:false")
				_ = measures.Incr(s)
				continue
			}
			zctx.Info("successfully run collection")
			s.Tags = append(s.Tags, "success:true")
			_ = measures.Incr(s)
		}
	}
}
