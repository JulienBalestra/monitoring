package dnsqueries

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/miekg/dns"
)

const (
	CollectorDnsMasqName = "dnsmasq-queries"

	optionDNSMasqAddress = "address"
)

type Queries struct {
	conf     *collector.Config
	measures *metrics.Measures

	dnsClient           *dns.Client
	dnsCounterQuestions map[string]dns.Question
	dnsGaugeQuestions   map[string]dns.Question
}

func NewDNSMasqQueries(conf *collector.Config) collector.Collector {
	return &Queries{
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
				Name:   exported.HitsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.miss": {
				Name:   exported.MissesQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.eviction": {
				Name:   exported.EvictionsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
			"dnsmasq.dns.cache.insertion": {
				Name:   exported.InsertionsQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
		dnsGaugeQuestions: map[string]dns.Question{
			"dnsmasq.dns.cache.size": {
				Name:   exported.CachesizeQueryBind + ".",
				Qtype:  dns.TypeTXT,
				Qclass: dns.ClassCHAOS,
			},
		},
	}
}

func (c *Queries) DefaultOptions() map[string]string {
	return map[string]string{
		optionDNSMasqAddress: "127.0.0.1:53",
	}
}

func (c *Queries) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Queries) IsDaemon() bool { return false }

func (c *Queries) Config() *collector.Config {
	return c.conf
}

func (c *Queries) Name() string {
	return CollectorDnsMasqName
}

func (c *Queries) Collect(ctx context.Context) error {
	now := time.Now()
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	for metricName, dnsQuestion := range c.dnsCounterQuestions {
		v, err := c.queryDnsmasqMetric(ctx, &dnsQuestion)
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
		v, err := c.queryDnsmasqMetric(ctx, &dnsQuestion)
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

func (c *Queries) queryDnsmasqMetric(ctx context.Context, question *dns.Question) (float64, error) {
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

	in, _, err := c.dnsClient.ExchangeContext(ctx, msg, address)
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
