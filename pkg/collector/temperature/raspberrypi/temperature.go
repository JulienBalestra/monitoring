package raspberrypi

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
	CollectorTemperatureName = "temperature-rpi"

	optionTemperatureFile = "temperature-file"
)

type Temperature struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewTemperature(conf *collector.Config) collector.Collector {
	return &Temperature{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Temperature) DefaultOptions() map[string]string {
	return map[string]string{
		optionTemperatureFile: "/sys/class/thermal/thermal_zone0/temp",
	}
}

func (c *Temperature) DefaultCollectInterval() time.Duration {
	return time.Minute * 2
}

func (c *Temperature) Config() *collector.Config {
	return c.conf
}

func (c *Temperature) IsDaemon() bool { return false }

func (c *Temperature) Name() string {
	return CollectorTemperatureName
}

func (c *Temperature) Collect(_ context.Context) error {
	tempFile, ok := c.conf.Options[optionTemperatureFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionTemperatureFile))
		return errors.New("missing option " + optionTemperatureFile)
	}

	divideBy := 1000.

	// example content:
	// 37552
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
		Name:      "temperature.celsius",
		Value:     t,
		Timestamp: time.Now(),
		Host:      c.conf.Host,
		Tags:      append(c.conf.Tagger.GetUnstableWithDefault(c.conf.Host), "sensor:cpu"),
	}, c.conf.CollectInterval*3)
	return nil
}
