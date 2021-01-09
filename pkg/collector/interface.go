package collector

import (
	"context"
	"strconv"
	"time"

	"github.com/JulienBalestra/dry/pkg/ticknow"
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
	Tags            []string
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
	DefaultTags() []string
	Tags() []string
}

func RunCollection(ctx context.Context, c Collector) error {
	// manage defaults
	config := c.Config()
	if config.Options == nil {
		config.Options = c.DefaultOptions()
	}
	if config.CollectInterval == 0 {
		config.CollectInterval = c.DefaultCollectInterval()
	}
	if config.Tags == nil {
		config.Tags = c.DefaultTags()
	}

	zctx := zap.L().With(
		zap.String("co", c.Name()),
	)
	extCtx := zctx.With(
		zap.Duration("collectionInterval", config.CollectInterval),
		zap.Any("options", config.Options),
	)

	if c.IsDaemon() {
		extCtx.Info("collecting metrics continuously")
		err := c.Collect(ctx)
		if err != nil {
			return err
		}
		return nil
	}

	ticker := ticknow.NewTickNow(ctx, config.CollectInterval)
	extCtx.Info("collecting metrics periodically")
	collectorTag := "collector:" + c.Name()
	measures := metrics.NewMeasures(config.MetricsClient.ChanSeries)
	for {
		select {
		case <-ctx.Done():
			extCtx.Info("end of collection")
			return ctx.Err()

		case <-ticker.C:
			now := time.Now()
			err := c.Collect(ctx)
			_ = measures.Incr(&metrics.Sample{
				Name:  "collector.collections",
				Value: 1,
				Time:  now,
				Host:  config.Host,
				Tags: append(config.Tagger.GetUnstable(config.Host),
					collectorTag,
					"success:"+strconv.FormatBool(err == nil),
				),
			})
			if err != nil {
				extCtx.Error("failed collection", zap.Error(err))
				continue
			}
			zctx.Info("ok")
		}
	}
}
