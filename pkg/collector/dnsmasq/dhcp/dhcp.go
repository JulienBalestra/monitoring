package dhcp

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/macvendor"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorName = "dnsmasq-dhcp"

	optionDNSMasqLeaseFile = "leases-file"

	dhcpWildcardLeaseValue = "wildcard"
)

/* cat /tmp/dnsmasq.leases
1586873170 cc:61:e5:8f:78:ea 192.168.1.149 android-f1703c3606a2892d 01:cc:61:e5:8f:78:ea
1586870968 90:78:b2:5c:07:af 192.168.1.148 Mi9T-MiPhone 01:90:78:b2:5c:07:af
1586869194 b8:8a:ec:fa:76:59 192.168.1.101 * *
1586868169 b0:2a:43:1e:62:99 192.168.1.134 Chromecast *
1586868164 f0:ef:86:2b:0e:89 192.168.1.147 Google-Nest-Mini *
1586868164 b4:e6:2d:0a:6b:97 192.168.1.135 ESP_0A6B97 *
1586868164 5c:cf:7f:9a:91:9a 192.168.1.121 ESP_9A919A *
1586868164 60:01:94:4e:dd:8a 192.168.1.114 ESP_4EDD8A *
*/

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	splitSep []byte
}

func NewDNSMasqDHCP(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		splitSep: []byte{'\n'},
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
		optionDNSMasqLeaseFile: "/tmp/dnsmasq.leases",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	dnsmasqFile, ok := c.conf.Options[optionDNSMasqLeaseFile]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionDNSMasqLeaseFile))
		return errors.New("missing option " + optionDNSMasqLeaseFile)
	}
	b, err := ioutil.ReadFile(dnsmasqFile)
	if err != nil {
		return err
	}

	if len(b) == 0 {
		zap.L().Debug("dnsmasq file is empty",
			zap.ByteString(dnsmasqFile, b),
		)
		return nil
	}
	lines := bytes.Split(b[:len(b)-1], c.splitSep)
	if len(lines) == 0 {
		zap.L().Debug("dnsmasq lease file is empty",
			zap.ByteString(dnsmasqFile, b),
		)
		return nil
	}
	now := time.Now()
	timestampSeconds := float64(now.Unix())
	tags := c.Tags()
	for _, line := range lines {
		raw := bytes.Fields(line)
		if len(raw) != 5 {
			zap.L().Error("failed to parse dnsmasq line",
				zap.ByteString("line", line),
				zap.Int("len", len(raw)),
				zap.ByteStrings("fields", raw),
			)
			continue
		}

		lease, macAddress, ipAddress, leaseName := string(raw[0]), string(raw[1]), string(raw[2]), string(raw[3])
		leaseStarted, err := strconv.ParseFloat(lease, 10)
		if err != nil {
			zap.L().Error("failed to parse dnsmasq line",
				zap.Error(err),
			)
			continue
		}
		macAddress = strings.ReplaceAll(macAddress, ":", "-")
		macAddressTag := tagger.NewTagUnsafe("mac", macAddress)
		vendorTag := tagger.NewTagUnsafe("vendor", macvendor.GetVendorWithMacOrUnknown(macAddress))
		ipAddressTag := tagger.NewTagUnsafe("ip", ipAddress)
		leaseNameTag := tagger.NewTagUnsafe(exported.LeaseKey, leaseName)
		if leaseName == "*" {
			leaseNameTag = tagger.NewTagUnsafe(exported.LeaseKey, dhcpWildcardLeaseValue)
			c.conf.Tagger.Update(ipAddress, macAddressTag, vendorTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag, vendorTag)
		} else {
			c.conf.Tagger.Update(leaseName, macAddressTag, ipAddressTag, vendorTag)
			c.conf.Tagger.Update(ipAddress, leaseNameTag, macAddressTag, vendorTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag, leaseNameTag, vendorTag)
		}
		c.measures.Gauge(&metrics.Sample{
			Name:  "dnsmasq.dhcp.lease",
			Value: leaseStarted - timestampSeconds,
			Time:  now,
			Host:  c.conf.Host,
			Tags: append(tags,
				leaseNameTag.String(),
				macAddressTag.String(),
				ipAddressTag.String(),
				vendorTag.String(),
			),
		})
	}
	c.measures.Purge()
	return nil
}
