package dnsmasq

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/miekg/dns"
)

const (
	CollectorDnsMasqName = "dnsmasq"

	optionDNSMasqLeaseFile = "leases-file"
	optionDNSMasqAddress   = "address"

	dhcpWildcardLeaseValue = "wildcard"

	dnsQueryBindSuffix  = ".bind"
	hitsQueryBind       = "hits" + dnsQueryBindSuffix
	missesQueryBind     = "misses" + dnsQueryBindSuffix
	evictionsQueryBind  = "evictions" + dnsQueryBindSuffix
	insertionsQueryBind = "insertions" + dnsQueryBindSuffix
	cachesizeQueryBind  = "cachesize" + dnsQueryBindSuffix
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

type DnsMasq struct {
	conf     *collector.Config
	measures *metrics.Measures

	dnsClient           *dns.Client
	dnsCounterQuestions map[string]dns.Question
	dnsGaugeQuestions   map[string]dns.Question

	splitSep []byte
}

func NewDnsMasq(conf *collector.Config) collector.Collector {
	return &DnsMasq{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		dnsClient: &dns.Client{
			Timeout:      time.Second,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		dnsCounterQuestions: map[string]dns.Question{
			"dnsmasq.dns.cache.hit": {
				Name:   hitsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.miss": {
				Name:   missesQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.eviction": {
				Name:   evictionsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.insertion": {
				Name:   insertionsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
		dnsGaugeQuestions: map[string]dns.Question{
			"dnsmasq.dns.cache.size": {
				Name:   cachesizeQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
		splitSep: []byte{'\n'},
	}
}

func (c *DnsMasq) DefaultOptions() map[string]string {
	return map[string]string{
		optionDNSMasqLeaseFile: "/tmp/dnsmasq.leases",
		optionDNSMasqAddress:   "127.0.0.1:53",
	}
}

func (c *DnsMasq) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *DnsMasq) IsDaemon() bool { return false }

func (c *DnsMasq) Config() *collector.Config {
	return c.conf
}

func (c *DnsMasq) Name() string {
	return CollectorDnsMasqName
}

func (c *DnsMasq) Collect(_ context.Context) error {
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
	hostTags := c.conf.Tagger.Get(c.conf.Host)
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
		macAddressTag, ipAddressTag, leaseNameTag := tagger.NewTagUnsafe("mac", macAddress), tagger.NewTagUnsafe("ip", ipAddress), tagger.NewTagUnsafe(exported.LeaseKey, leaseName)
		if leaseName == "*" {
			leaseNameTag = tagger.NewTagUnsafe(exported.LeaseKey, dhcpWildcardLeaseValue)
			c.conf.Tagger.Update(ipAddress, macAddressTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag)
		} else {
			c.conf.Tagger.Update(leaseName, macAddressTag, ipAddressTag)
			c.conf.Tagger.Update(ipAddress, leaseNameTag, macAddressTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag, leaseNameTag)
		}
		c.measures.Gauge(&metrics.Sample{
			Name:      "dnsmasq.dhcp.lease",
			Value:     leaseStarted - timestampSeconds,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      append(hostTags, leaseNameTag.String(), macAddressTag.String(), ipAddressTag.String()),
		})
	}

	for metricName, dnsQuestion := range c.dnsCounterQuestions {
		v, err := c.queryDnsmasqMetric(&dnsQuestion)
		if err != nil {
			zap.L().Error("failed to query dnsmasq",
				zap.Error(err),
				zap.String("question", dnsQuestion.Name),
			)
			continue
		}
		_ = c.measures.Count(&metrics.Sample{
			Name:      metricName,
			Value:     v,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		})
	}
	for metricName, dnsQuestion := range c.dnsGaugeQuestions {
		v, err := c.queryDnsmasqMetric(&dnsQuestion)
		if err != nil {
			zap.L().Error("failed to query dnsmasq",
				zap.Error(err),
				zap.String("question", dnsQuestion.Name),
			)
			continue
		}
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:      metricName,
			Value:     v,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		}, time.Minute*30)
	}
	c.measures.Purge()
	return nil
}

func (c *DnsMasq) queryDnsmasqMetric(question *dns.Question) (float64, error) {
	address, ok := c.conf.Options[optionDNSMasqAddress]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionDNSMasqAddress))
		return 0, errors.New("missing option " + optionDNSMasqAddress)
	}
	msg := &dns.Msg{
		Question: []dns.Question{*question},
	}
	msg.Id = dns.Id()
	msg.RecursionDesired = true

	in, _, err := c.dnsClient.Exchange(msg, address)
	if err != nil {
		return 0, err
	}
	if len(in.Answer) != 1 {
		return 0, fmt.Errorf("invalid number of Answer records: %d", len(in.Answer))
	}
	t, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		return 0, errors.New("not a TXT")
	}
	if len(t.Txt) != 1 {
		return 0, fmt.Errorf("invalid number of TXT records: %d", len(t.Txt))
	}
	return strconv.ParseFloat(t.Txt[0], 10)
}
