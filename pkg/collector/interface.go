package collector

import (
	"context"
	"time"

	"github.com/JulienBalestra/dry/pkg/ticknow"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	collectorMetricPrefix = "collector."
)

type Config struct {
	MetricsClient *datadog.Client
	Tagger        *tagger.Tagger

	Host            string
	CollectInterval time.Duration
	Options         map[string]string
	Tags            []string
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
	SubmittedSeries() float64
}

func WithDefaults(c Collector) Collector {
	config := c.Config()

	if config.CollectInterval == 0 {
		config.CollectInterval = c.DefaultCollectInterval()
	}

	if config.Options == nil {
		config.Options = c.DefaultOptions()
	} else {
		d := c.DefaultOptions()
		for k, v := range d {
			_, ok := config.Options[k]
			if ok {
				continue
			}
			config.Options[k] = v
		}
	}

	if config.Tags == nil {
		config.Tags = c.DefaultTags()
	} else {
		m := make(map[string]struct{}, len(config.Tags))
		for _, t := range config.Tags {
			m[t] = struct{}{}
		}
		for _, d := range c.DefaultTags() {
			_, ok := m[d]
			if ok {
				continue
			}
			config.Tags = append(config.Tags, d)
		}
	}

	return c
}

func RunCollection(ctx context.Context, c Collector) error {
	config := c.Config()

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

	runCollection := time.NewTicker(config.CollectInterval)
	extCtx.Info("collecting metrics periodically")
	collectorTag := "collector:" + c.Name()
	measures := metrics.NewMeasures(config.MetricsClient.ChanSeries)

	collectorMetrics := ticknow.NewTickNowWithContext(ctx, time.Minute*5)
	var series, runSuccess, runErr float64
	for {
		select {
		case <-ctx.Done():
			runCollection.Stop()
			extCtx.Info("end of collection")
			return ctx.Err()

		case <-collectorMetrics.C:
			now := time.Now()
			_ = measures.Count(&metrics.Sample{
				Name:  collectorMetricPrefix + "series",
				Value: series,
				Time:  now,
				Host:  config.Host,
				Tags:  append(config.Tagger.GetUnstable(config.Host), collectorTag),
			})
			_ = measures.Count(&metrics.Sample{
				Name:  collectorMetricPrefix + "collections",
				Value: runSuccess,
				Time:  now,
				Host:  config.Host,
				Tags:  append(config.Tagger.GetUnstable(config.Host), collectorTag, "success:true"),
			})
			_ = measures.Count(&metrics.Sample{
				Name:  collectorMetricPrefix + "collections",
				Value: runErr,
				Time:  now,
				Host:  config.Host,
				Tags:  append(config.Tagger.GetUnstable(config.Host), collectorTag, "success:false"),
			})

		case <-runCollection.C:
			beforeCollection := c.SubmittedSeries()
			// TODO observe performance
			err := c.Collect(ctx)
			series = c.SubmittedSeries()
			collectionSeries := series - beforeCollection
			if err != nil {
				runErr++
				extCtx.Error("failed collection", zap.Error(err), zap.Float64("series", collectionSeries))
				continue
			}
			runSuccess++
			zctx.Info("ok", zap.Uint64("s", uint64(collectionSeries)))
		}
	}
}
