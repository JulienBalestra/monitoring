package arp

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/mac"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	exportedTags "github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorName = "network-arp"

	optionARPFile = "arp-file"
)

/* cat /proc/self/net/arp
IP address       HW type     Flags       HW address            Mask     Device
192.168.1.114    0x1         0x2         60:01:94:4e:dd:8a     *        br0
192.168.1.135    0x1         0x2         b4:e6:2d:0a:6b:97     *        br0
78.194.245.254   0x1         0x2         68:a3:78:61:f0:81     *        vlan2
192.168.1.121    0x1         0x2         5c:cf:7f:9a:91:9a     *        br0
192.168.1.123    0x1         0x2         a0:ce:c8:d3:55:bd     *        br0
192.168.1.147    0x1         0x2         f0:ef:86:2b:0e:89     *        br0
192.168.1.134    0x1         0x2         b0:2a:43:1e:62:99     *        br0
*/

type ARP struct {
	conf     *collector.Config
	measures *metrics.Measures
	leaseTag *tagger.Tag
}

func NewARP(conf *collector.Config) collector.Collector {
	return &ARP{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		leaseTag: tagger.NewTagUnsafe(exportedTags.LeaseKey, tagger.MissingTagValue),
	}
}

func (c *ARP) DefaultOptions() map[string]string {
	return map[string]string{
		optionARPFile: "/proc/self/net/arp",
	}
}

func (c *ARP) DefaultCollectInterval() time.Duration {
	return time.Second * 10
}

func (c *ARP) Config() *collector.Config {
	return c.conf
}

func (c *ARP) IsDaemon() bool { return false }

func (c *ARP) Name() string {
	return CollectorName
}

func (c *ARP) Collect(_ context.Context) error {
	arpFile, ok := c.conf.Options[optionARPFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionARPFile))
		return errors.New("missing option " + optionARPFile)
	}

	b, err := ioutil.ReadFile(arpFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(b[:len(b)-1]), "\n")
	if len(lines) == 0 {
		return nil
	}
	now := time.Now()
	hostTags := c.conf.Tagger.GetUnstable(c.conf.Host)
	for _, line := range lines[1:] {
		raw := strings.Fields(line)
		if len(raw) != 6 {
			zap.L().Error("failed to parse arp line", zap.String("line", line), zap.Strings("fields", raw))
			continue
		}
		ipAddress, macAddress, device := raw[0], raw[3], raw[5]
		if macAddress == "00:00:00:00:00:00" {
			zap.L().Debug("ignoring entry",
				zap.String("ip", ipAddress),
				zap.String("mac", macAddress),
				zap.String("device", device),
			)
			continue
		}
		macAddress = strings.ReplaceAll(macAddress, ":", "-")
		macAddressTag, ipAddressTag, deviceTag := tagger.NewTagUnsafe("mac", macAddress), tagger.NewTagUnsafe("ip", ipAddress), tagger.NewTagUnsafe("device", device)
		vendorTag := tagger.NewTagUnsafe("vendor", mac.GetVendorWithMacOrUnknown(macAddress))
		c.conf.Tagger.Update(ipAddress, macAddressTag, deviceTag, vendorTag)
		c.conf.Tagger.Update(macAddress, ipAddressTag, deviceTag, vendorTag)

		// we rely on dnsmasq tags collection to make this available
		tags := append(hostTags, c.conf.Tagger.GetUnstableWithDefault(macAddress, c.leaseTag)...)
		tags = append(tags, deviceTag.String(), macAddressTag.String())
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:  "network.arp",
			Value: 1,
			Time:  now,
			Host:  c.conf.Host,
			Tags:  tags,
		}, c.conf.CollectInterval*3)
	}
	c.measures.Purge()
	return nil
}
