package ping

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"go.uber.org/zap"
)

const (
	CollectorName = "ping"

	OptionTarget  = "target"
	OptionTimeout = "timeout"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	timeStart []byte
	timeEnd   []byte
}

func NewPing(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:      conf,
		measures:  metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		timeStart: []byte("time="),
		timeEnd:   []byte(" ms"),
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
		OptionTarget:  "1.1.1.1",
		OptionTimeout: "2s",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Minute
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(ctx context.Context) error {
	target, ok := c.conf.Options[OptionTarget]
	if !ok {
		zap.L().Error("missing option",
			zap.String("options", OptionTarget),
		)
		return errors.New("missing option " + OptionTarget)
	}
	timeout, ok := c.conf.Options[OptionTimeout]
	if !ok {
		zap.L().Error("missing option",
			zap.String("options", OptionTimeout),
		)
		return errors.New("missing option " + OptionTimeout)
	}
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zap.L().Error("invalid option",
			zap.String("options", OptionTimeout),
			zap.String(OptionTimeout, timeout),
			zap.Error(err),
		)
		return err
	}
	if timeoutDuration >= c.conf.CollectInterval {
		err := fmt.Errorf("must be lower than the collection interval: %s < %s", c.conf.CollectInterval.String(), timeoutDuration.String())
		zap.L().Error("invalid option",
			zap.String("options", OptionTimeout),
			zap.String(OptionTimeout, timeout),
			zap.String("collectInterval", c.conf.CollectInterval.String()),
			zap.Error(err),
		)
		return err
	}

	dst, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	b, err := exec.CommandContext(ctx, "ping",
		"-w", timeout,
		"-W", timeout,
		"-c", "1",
		dst.IP.String()).CombinedOutput()
	if err != nil {
		return err
	}
	i := bytes.Index(b, c.timeStart)
	if i == -1 {
		return errors.New("failed to parse ping output")
	}
	i += 5
	b = b[i:]
	i = bytes.Index(b, c.timeEnd)
	if i == -1 {
		return errors.New("failed to parse ping output")
	}
	f, err := strconv.ParseFloat(string(b[:i]), 10)
	if err != nil {
		return err
	}
	tags := append(c.Tags(), "ip:"+dst.IP.String(), "target:"+target)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "latency.icmp",
		Value: f,
		Time:  time.Now(),
		Host:  c.conf.Host,
		Tags:  append(tags, c.conf.Tagger.GetUnstable(target)...),
	}, c.conf.CollectInterval*3)
	return nil
}
