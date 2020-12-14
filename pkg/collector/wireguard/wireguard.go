package wireguard

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorWireguardName = "wireguard"

	optionWgFile = "wg-file"

	wireguardMetricPrefix = "wireguard."
)

/*
Built with:
wg --version
wireguard-tools v1.0.20200827 - https://git.zx2c4.com/wireguard-tools/
*/

type Wireguard struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewWireguard(conf *collector.Config) collector.Collector {
	return &Wireguard{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Wireguard) DefaultOptions() map[string]string {
	return map[string]string{
		optionWgFile: "/usr/bin/wg",
	}
}

func (c *Wireguard) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Wireguard) Config() *collector.Config {
	return c.conf
}

func (c *Wireguard) IsDaemon() bool { return false }

func (c *Wireguard) Name() string {
	return CollectorWireguardName
}

func (c *Wireguard) wgShow(ctx context.Context, interfaceName, display string) ([]string, error) {
	var line []string
	b, err := exec.CommandContext(ctx, c.conf.Options[optionWgFile], "show", interfaceName, display).CombinedOutput()
	if err != nil {
		return line, err
	}
	if b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return strings.Split(string(b), "\n"), nil
}

func (c *Wireguard) Collect(ctx context.Context) error {
	wgExec, ok := c.conf.Options[optionWgFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionWgFile))
		return errors.New("missing option " + optionWgFile)
	}
	b, err := exec.CommandContext(ctx, wgExec, "show", "interfaces").CombinedOutput()
	if err != nil {
		return err
	}
	output := string(b)
	var interfaces []string
	for _, i := range strings.Split(output, "\n") {
		if i == "" {
			continue
		}
		interfaces = append(interfaces, i)
	}

	for _, interfaceName := range interfaces {
		/*
			wg show wg0 endpoints
			5+gsKzJeB8+sOqJ+yL/ItneCZfI8O2cEaOG5Gc3HHlQ=	192.168.2.251:51871
		*/
		lines, err := c.wgShow(ctx, interfaceName, "endpoints")
		if err != nil {
			return err
		}
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				zap.L().Warn("incoherent output of endpoints", zap.String("line", line))
				continue
			}
			peerID, endpoint := fields[0], fields[1]
			index := strings.IndexByte(endpoint, ':')
			ip := endpoint
			if index != -1 {
				ip = endpoint[:index]
			}
			c.conf.Tagger.Update(peerID,
				tagger.NewTagUnsafe("endpoint", endpoint),
				tagger.NewTagUnsafe("ip", ip),
			)
			c.conf.Tagger.Update(ip,
				tagger.NewTagUnsafe("peer-id", peerID),
				tagger.NewTagUnsafe("endpoint", endpoint),
			)
		}

		/*
			wg show wg0 transfer
			5+gsKzJeB8+sOqJ+yL/ItneCZfI8O2cEuOG5Gn3HHlQ=	14513347932	343873344
		*/
		lines, err = c.wgShow(ctx, interfaceName, "transfer")
		if err != nil {
			return err
		}
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				zap.L().Warn("incoherent output of transfer", zap.String("line", line))
				continue
			}
			peerID, received, sent := fields[0], fields[1], fields[2]
			receivedBytes, err := strconv.ParseFloat(received, 10)
			if err != nil {
				zap.L().Error("cannot parse received from transfer", zap.String("line", line), zap.Error(err))
				continue
			}
			sentBytes, err := strconv.ParseFloat(sent, 10)
			if err != nil {
				zap.L().Error("cannot parse received from transfer", zap.String("line", line), zap.Error(err))
				continue
			}
			now := time.Now()
			tags := c.conf.Tagger.GetUnstable(peerID)
			_ = c.measures.Count(&metrics.Sample{
				Name:      wireguardMetricPrefix + "transfer.received",
				Value:     receivedBytes,
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
			_ = c.measures.Count(&metrics.Sample{
				Name:      wireguardMetricPrefix + "transfer.sent",
				Value:     sentBytes,
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
		}
	}
	return nil
}
