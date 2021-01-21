package ddwrt

import (
	"context"
	"errors"
	"io/ioutil"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/metrics"

	"github.com/JulienBalestra/monitoring/pkg/collector"
)

const (
	CollectorName = "temperature-dd-wrt"

	optionTemperatureFile = "temperature-file"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewTemperature(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
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
		optionTemperatureFile: "/proc/dmu/temperature",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	tempFile, ok := c.conf.Options[optionTemperatureFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionTemperatureFile))
		return errors.New("missing option " + optionTemperatureFile)
	}

	divideBy := 10.
	// example content:
	// 669
	temp, err := ioutil.ReadFile(tempFile)
	if err != nil {
		return err
	}

	t, err := strconv.ParseFloat(string(temp[:len(temp)-1]), 10)
	if err != nil {
		return err
	}
	t /= divideBy

	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "temperature.celsius",
		Value: t,
		Time:  time.Now(),
		Host:  c.conf.Host,
		Tags:  append(c.conf.Tagger.GetUnstableWithDefault(c.conf.Host), "sensor:cpu"),
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	return nil
}
