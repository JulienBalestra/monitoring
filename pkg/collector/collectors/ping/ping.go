package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/JulienBalestra/dry/pkg/fnv"
	"go.uber.org/zap"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "ping"

	OptionTarget = "target"
	OptionListen = "listen"

	listenOnAllAddresses = "0.0.0.0"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	pid   uint64
	reply []byte
}

func NewPing(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		pid:      uint64(os.Getpid()),
		reply:    make([]byte, 1500),
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
		OptionListen: listenOnAllAddresses,
		OptionTarget: "1.1.1.1",
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

func (c *Collector) ping(ctx context.Context, destination *net.IPAddr, listen string) (time.Duration, error) {
	l, err := icmp.ListenPacket("ip4:icmp", listen)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	err = l.SetDeadline(time.Now().Add(c.conf.CollectInterval))
	if err != nil {
		return 0, err
	}

	h := fnv.NewHash()
	h = fnv.Add(h, c.pid)
	h = fnv.AddString(h, destination.String())
	m := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:  int(h),
			Seq: 1,
		},
	}
	b, err := m.Marshal(nil)
	if err != nil {
		return 0, err
	}

	resultCh := make(chan time.Duration)
	defer close(resultCh)
	errCh := make(chan error)
	defer close(errCh)
	go func() {
		start := time.Now()
		_, err = l.WriteTo(b, destination)
		if err != nil {
			errCh <- err
			return
		}
		n, _, err := l.ReadFrom(c.reply)
		if err != nil {
			errCh <- err
			return
		}
		duration := time.Since(start)
		rm, err := icmp.ParseMessage(1, c.reply[:n])
		if err != nil {
			errCh <- err
			return
		}
		if rm.Type != ipv4.ICMPTypeEchoReply {
			errCh <- fmt.Errorf("wrong ping response: %v", rm.Type)
			return
		}
		resultCh <- duration
	}()

	for {
		select {
		case <-ctx.Done():
			_ = l.SetDeadline(time.Now().Add(0))

		case err := <-errCh:
			return 0, err

		case r := <-resultCh:
			return r, nil
		}
	}
}

func (c *Collector) Collect(ctx context.Context) error {
	target, ok := c.conf.Options[OptionTarget]
	if !ok {
		zap.L().Error("missing option", zap.String("options", OptionTarget))
		return errors.New("missing option " + OptionTarget)
	}

	dst, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return err
	}

	la, ok := c.conf.Options[OptionListen]
	if !ok {
		la = listenOnAllAddresses
		zap.L().Debug("defaulting option",
			zap.String("option", OptionListen),
			zap.String(OptionListen, listenOnAllAddresses),
		)
	}

	d, err := c.ping(ctx, dst, la)
	if err != nil {
		zap.L().Error("failed to ping",
			zap.Error(err),
			zap.String(OptionListen, la),
			zap.String(OptionTarget, target),
			zap.String("ip", dst.IP.String()),
		)
		return err // this could be noisy
	}
	tags := append(c.Tags(), "ip:"+dst.IP.String(), "target:"+target)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "latency.icmp",
		Value: d.Seconds(),
		Time:  time.Now(),
		Host:  c.conf.Host,
		Tags:  append(tags, c.conf.Tagger.GetUnstable(target)...),
	}, c.conf.CollectInterval*3)
	return nil
}
