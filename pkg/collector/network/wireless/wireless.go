package wireless

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "network-wireless"

	wirelessMetricPrefix       = "network.wireless."
	wirelessDiscardRetryMetric = wirelessMetricPrefix + "discard.retry"

	optionSysClassPath = "sys-class-net-path"
	optionWirelessFile = "proc-net-wireless-file"
)

/*
cat /proc/net/wireless
Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
  eth1: 0000    5.  -256.  -84.       0      4      0   1413      0        0
  eth2: 0000    5.  -256.  -92.       0     15      0    656     14        0

*/

type Wireless struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewWireless(conf *collector.Config) collector.Collector {
	return &Wireless{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Wireless) DefaultOptions() map[string]string {
	return map[string]string{
		optionSysClassPath: "/sys/class/net/",
		optionWirelessFile: "/proc/net/wireless",
	}
}

func (c *Wireless) DefaultCollectInterval() time.Duration {
	return time.Second * 10
}

func (c *Wireless) Config() *collector.Config {
	return c.conf
}

func (c *Wireless) IsDaemon() bool { return false }

func (c *Wireless) Name() string {
	return CollectorName
}

func (c *Wireless) Collect(_ context.Context) error {
	wirelessPath, ok := c.conf.Options[optionWirelessFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionWirelessFile))
		return errors.New("missing option " + optionWirelessFile)
	}

	sysClassPath, ok := c.conf.Options[optionSysClassPath]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionSysClassPath))
		return errors.New("missing option " + optionSysClassPath)
	}

	file, err := os.Open(wirelessPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	now := time.Now()
	l := 0
	for {
		// TODO improve this reader
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if l < 2 {
			l++
			continue
		}
		fields := bytes.Fields(line)
		//                                    eth1:
		//                                        ^
		device, noise, discardRetry := string(fields[0][:len(fields[0])-1]), string(fields[4]), string(fields[8])

		deviceMac, err := ioutil.ReadFile(sysClassPath + device + "/address")
		if err != nil {
			zap.L().Error("failed to parse device", zap.Error(err))
			continue
		}
		deviceMacR := strings.ReplaceAll(string(deviceMac), ":", "-")
		tags := append(hostTags, "device:"+device, "mac:"+deviceMacR)

		noiseV, err := strconv.ParseFloat(noise, 10)
		if err != nil {
			zap.L().Error("failed to parse noise", zap.Error(err))
			continue
		}
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:      wirelessMetricPrefix + "noise",
			Value:     noiseV,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		}, c.conf.CollectInterval*3)

		discardRetryV, err := strconv.ParseFloat(discardRetry, 10)
		if err != nil {
			zap.L().Error("failed to parse discard/retry", zap.Error(err))
			continue
		}
		_ = c.measures.Count(&metrics.Sample{
			Name:      wirelessDiscardRetryMetric,
			Value:     discardRetryV,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
	}
	return nil
}
