package network

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collector"
	exportedTags "github.com/JulienBalestra/metrics/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

const (
	arpPath = "/proc/self/net/arp"
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
	conf *collector.Config
}

func NewARPReporter(conf *collector.Config) *ARP {
	return &ARP{
		conf: conf,
	}
}

func (c *ARP) Config() *collector.Config {
	return c.conf
}

func (c *ARP) Name() string {
	if c.conf.CollectorName != "" {
		return c.conf.CollectorName
	}
	return "network/arp"
}

func (c *ARP) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	b, err := ioutil.ReadFile(arpPath)
	if err != nil {
		return counters, gauges, err
	}

	lines := strings.Split(string(b[:len(b)-1]), "\n")
	if len(lines) == 0 {
		return counters, gauges, nil
	}
	now := time.Now()
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	for i, line := range lines[1:] {
		raw := strings.Fields(line)
		if len(raw) != 6 {
			log.Printf("failed to parse arp line %d len(%d): %q : %q", i, len(raw), line, strings.Join(raw, ","))
			continue
		}
		ipAddress, macAddress, device := raw[0], raw[3], raw[5]
		if macAddress == "00:00:00:00:00:00" {
			log.Printf("ignoring entry %s %s %s", ipAddress, macAddress, device)
			continue
		}
		macAddress = strings.ReplaceAll(macAddress, ":", "-")
		macAddressTag, ipAddressTag, deviceTag := tagger.NewTag("mac", macAddress), tagger.NewTag("ip", ipAddress), tagger.NewTag("device", device)
		c.conf.Tagger.Update(ipAddress, macAddressTag, deviceTag)
		c.conf.Tagger.Update(macAddress, ipAddressTag, deviceTag)

		// we rely on dnsmasq tags collection to make this available
		tags := append(hostTags, c.conf.Tagger.GetWithDefault(macAddress, tagger.NewTag(exportedTags.LeaseKey, tagger.MissingTagValue))...)
		tags = append(tags, deviceTag.String(), macAddressTag.String())
		gauges = append(gauges, &datadog.Metric{
			Name:      "network.arp",
			Value:     1,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
	}

	return counters, gauges, nil
}
