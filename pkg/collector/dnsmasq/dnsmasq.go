package dnsmasq

import (
	"context"
	"errors"
	"fmt"
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	dnsmasqPath = "/tmp/dnsmasq.leases"

	dhcpWildcardLeaseTag = exported.LeaseKey + ":" + "wildcard"
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
	conf *collector.Config

	dnsClient           *dns.Client
	dnsCounterQuestions map[string]dns.Question
	dnsGaugeQuestions   map[string]dns.Question
}

func NewDnsMasqReporter(conf *collector.Config) *DnsMasq {
	return &DnsMasq{
		conf: conf,
		dnsClient: &dns.Client{
			Timeout:      time.Second,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		dnsCounterQuestions: map[string]dns.Question{
			"dnsmasq.dns.cache.hit": {
				Name:   "hits.bind.",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.miss": {
				Name:   "misses.bind.",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.eviction": {
				Name:   "evictions.bind.",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.insertion": {
				Name:   "insertions.bind.",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
		dnsGaugeQuestions: map[string]dns.Question{
			"dnsmasq.dns.cache.size": {
				Name:   "cachesize.bind.",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
	}
}

func (c *DnsMasq) Config() *collector.Config {
	return c.conf
}

func (c *DnsMasq) Name() string {
	if c.conf.CollectorName != "" {
		return c.conf.CollectorName
	}
	return "dnsmasq"
}

func (c *DnsMasq) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	b, err := ioutil.ReadFile(dnsmasqPath)
	if err != nil {
		return counters, gauges, err
	}

	lines := strings.Split(string(b[:len(b)-1]), "\n")
	if len(lines) == 0 {
		return counters, gauges, nil
	}
	now := time.Now()
	timestampSeconds := float64(now.Unix())
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	for i, line := range lines {
		raw := strings.Fields(line)
		if len(raw) != 5 {
			log.Printf("failed to parse dnsmasq line %d len(%d): %q : %q", i, len(raw), line, strings.Join(raw, ","))
			continue
		}

		lease, macAddress, ipAddress, leaseName := raw[0], raw[1], raw[2], raw[3]
		leaseStarted, err := strconv.ParseFloat(lease, 10)
		if err != nil {
			log.Printf("failed to parse dnsmasq lease: %v", err)
			continue
		}
		macAddress = strings.ReplaceAll(macAddress, ":", "-")
		macAddressTag, ipAddressTag, leaseNameTag := tagger.NewTag("mac", macAddress), tagger.NewTag("ip", ipAddress), tagger.NewTag(exported.LeaseKey, leaseName)
		if leaseName == "*" {
			leaseNameTag = tagger.NewTag(exported.LeaseKey, "wildcard")
			c.conf.Tagger.Update(ipAddress, macAddressTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag)
		} else {
			c.conf.Tagger.Update(leaseName, macAddressTag, ipAddressTag)
			c.conf.Tagger.Update(ipAddress, leaseNameTag, macAddressTag)
			c.conf.Tagger.Update(macAddress, ipAddressTag, leaseNameTag)
		}

		gauges = append(gauges, &datadog.Metric{
			Name:      "dnsmasq.dhcp.lease",
			Value:     leaseStarted - timestampSeconds,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      append(hostTags, leaseNameTag.String(), macAddressTag.String(), ipAddressTag.String()),
		})
	}

	counters = make(datadog.Counter, len(c.dnsCounterQuestions))
	for metricName, dnsQuestion := range c.dnsCounterQuestions {
		v, err := c.queryDnsmasqMetric(&dnsQuestion)
		if err != nil {
			log.Printf("failed to query dnsmasq for %s: %v", dnsQuestion.Name, err)
			continue
		}
		counters[metricName] = &datadog.Metric{
			Name:      metricName,
			Value:     v,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		}
	}
	for metricName, dnsQuestion := range c.dnsGaugeQuestions {
		v, err := c.queryDnsmasqMetric(&dnsQuestion)
		if err != nil {
			log.Printf("failed to query dnsmasq for %s: %v", dnsQuestion.Name, err)
			continue
		}
		gauges = append(gauges, &datadog.Metric{
			Name:      metricName,
			Value:     v,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		})
	}
	return counters, gauges, nil
}

func (c *DnsMasq) queryDnsmasqMetric(question *dns.Question) (float64, error) {
	msg := &dns.Msg{
		Question: []dns.Question{*question},
	}
	msg.Id = dns.Id()
	msg.RecursionDesired = true

	in, _, err := c.dnsClient.Exchange(msg, "127.0.0.1:53") // TODO make this configurable
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
