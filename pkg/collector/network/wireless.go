package network

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorWirelessName = "network-wireless"

	wirelessPath               = "/proc/net/wireless"
	wirelessMetricPrefix       = "network.wireless."
	wirelessDiscardRetryMetric = wirelessMetricPrefix + "discard.retry"

	sysClassPath = "/sys/class/net/"
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
		measures: metrics.NewMeasures(conf.SeriesCh),
	}
}

func (c *Wireless) Config() *collector.Config {
	return c.conf
}

func (c *Wireless) IsDaemon() bool { return false }

func (c *Wireless) Name() string {
	return CollectorWirelessName
}

func (c *Wireless) Collect(_ context.Context) error {
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
			log.Printf("failed to parse device: %v", err)
			continue
		}
		deviceMacR := strings.ReplaceAll(string(deviceMac), ":", "-")
		tags := append(hostTags, "device:"+device, "mac:"+deviceMacR)

		noiseV, err := strconv.ParseFloat(noise, 10)
		if err != nil {
			log.Printf("failed to parse noise: %v", err)
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
			log.Printf("failed to parse discard/retry: %v", err)
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
