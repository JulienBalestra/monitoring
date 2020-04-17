package network

import (
	"bufio"
	"context"
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq/exported"
	selfExported "github.com/JulienBalestra/metrics/pkg/collector/network/exported"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"io"
	"os"
	"strings"
	"time"
)

const (
	conntrackPath = "/proc/net/ip_conntrack"
)

type Conntrack struct {
	conf *collector.Config
}

func NewConntrackReporter(conf *collector.Config) *Conntrack {
	return &Conntrack{
		conf: conf,
	}
}

func (c *Conntrack) Config() *collector.Config {
	return c.conf
}

func (c *Conntrack) Name() string {
	return "network/conntrack"
}

func (c *Conntrack) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	file, err := os.Open(conntrackPath)
	if err != nil {
		return counters, gauges, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	stats := make(map[string]*datadog.Metric)
	now := time.Now()
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		s := string(line)
		if !strings.HasPrefix(s, "tcp") {
			continue
		}
		fields := strings.Fields(s)
		state, src, dport := fields[3], fields[4], fields[7]
		src = strings.TrimPrefix(src, "src=")
		dport = strings.TrimPrefix(dport, "dport=")
		mapKey := src + state + dport
		st, ok := stats[mapKey]
		if !ok {
			tags := append(hostTags, c.conf.Tagger.GetWithDefault(src,
				tagger.NewTag(exported.LeaseKey, tagger.MissingTagValue),
				tagger.NewTag(selfExported.DeviceKey, tagger.MissingTagValue),
			)...)
			st = &datadog.Metric{
				Name:      "network.conntrack.tcp",
				Value:     0,
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      append(tags, "state:"+state, "src:"+src),
			}
			stats[mapKey] = st
		}
		st.Value++
	}
	for _, st := range stats {
		gauges = append(gauges, st)
	}
	return counters, gauges, nil
}
