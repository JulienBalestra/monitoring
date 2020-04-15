package network

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	exportedTags "github.com/JulienBalestra/metrics/pkg/collecter/dnsmasq/tags"
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
	conf *collecter.Config
}

func NewARPReporter(conf *collecter.Config) *ARP {
	return &ARP{
		conf: conf,
	}
}

func (c *ARP) collectMetrics() (datadog.GaugeList, error) {
	var gaugeLists datadog.GaugeList

	b, err := ioutil.ReadFile(arpPath)
	if err != nil {
		return gaugeLists, err
	}

	lines := strings.Split(string(b[:len(b)-1]), "\n")
	if len(lines) == 0 {
		return gaugeLists, nil
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
		macAddressTag, ipAddressTag := "mac:"+macAddress, "ip:"+ipAddress
		c.conf.Tagger.Upsert(ipAddress, macAddressTag)
		c.conf.Tagger.Upsert(macAddress, ipAddressTag)

		// we rely on dnsmasq tags collection to make this available
		tags := append(hostTags, c.conf.Tagger.GetWithDefault(macAddress, exportedTags.LeaseKey, tagger.MissingTagValue)...)
		tags = append(tags, "device:"+device, "mac:"+macAddress)
		gaugeLists = append(gaugeLists, &datadog.Metric{
			Name:      "network.arp",
			Value:     1,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
	}

	return gaugeLists, nil
}

func (c *ARP) Collect(ctx context.Context) {
	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting network/arp metrics every %s", c.conf.CollectInterval.String())
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of network/arp collection")
			return

		case <-ticker.C:
			gauges, err := c.collectMetrics()
			if err != nil {
				log.Printf("failed network/arp collection: %v", err)
				continue
			}
			gauges.Gauge(c.conf.MetricsCh)
			log.Printf("successfully run network/arp collection")
		}
	}
}
