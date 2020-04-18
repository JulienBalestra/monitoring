package network

import (
	"bufio"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq/exported"
	selfExported "github.com/JulienBalestra/metrics/pkg/collector/network/exported"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
)

const (
	CollectorConntrackName = "network-conntrack"

	conntrackPath = "/proc/net/ip_conntrack"
)

type Conntrack struct {
	conf *collector.Config
}

func NewConntrackReporter(conf *collector.Config) collector.Collector {
	return &Conntrack{
		conf: conf,
	}
}

func (c *Conntrack) Config() *collector.Config {
	return c.conf
}

func (c *Conntrack) Name() string {
	return CollectorConntrackName
}

func (c *Conntrack) parseTCPFields(fields []string, tcpStats map[string]*datadog.Metric) {
	state, srcIp, dstPort := fields[3], fields[4], fields[7]
	srcIp = strings.TrimPrefix(srcIp, "src=")
	dstPort = strings.TrimPrefix(dstPort, "dport=")
	mapKey := srcIp + state + dstPort
	st, ok := tcpStats[mapKey]
	if !ok {
		st = &datadog.Metric{
			Name: "network.conntrack.tcp",
			Host: c.conf.Host,
			Tags: append(c.conf.Tagger.GetWithDefault(srcIp,
				tagger.NewTag(exported.LeaseKey, tagger.MissingTagValue),
				tagger.NewTag(selfExported.DeviceKey, tagger.MissingTagValue),
			), "state:"+state, "src_ip:"+srcIp, "dst_port:"+dstPort),
		}
		tcpStats[mapKey] = st
	}
	st.Value++
}

func (c *Conntrack) parseUDPFields(fields []string, udpStats map[string]*datadog.Metric) error {
	srcIp, dstPort := fields[3], fields[6]
	srcIp = strings.TrimPrefix(srcIp, "src=")
	dstPort = strings.TrimPrefix(dstPort, "dport=")
	port, err := strconv.Atoi(dstPort)
	if err != nil {
		return err
	}
	if port > 1023 {
		dstPort = "1024+"
	}
	mapKey := srcIp + dstPort
	st, ok := udpStats[mapKey]
	if !ok {
		st = &datadog.Metric{
			Name: "network.conntrack.udp",
			Host: c.conf.Host,
			Tags: append(c.conf.Tagger.GetWithDefault(srcIp,
				tagger.NewTag(exported.LeaseKey, tagger.MissingTagValue),
				tagger.NewTag(selfExported.DeviceKey, tagger.MissingTagValue),
			), "src_ip:"+srcIp, "dst_port:"+dstPort),
		}
		udpStats[mapKey] = st
	}
	st.Value++
	return nil
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
	tcpStats := make(map[string]*datadog.Metric)
	udpStats := make(map[string]*datadog.Metric)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		s := string(line)
		fields := strings.Fields(s)
		if fields[0] == "tcp" {
			c.parseTCPFields(fields, tcpStats)
			continue
		}
		// udp
		_ = c.parseUDPFields(fields, udpStats)
	}
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	now := time.Now()
	for _, st := range tcpStats {
		st.Timestamp = now
		st.Tags = append(st.Tags, hostTags...)
		gauges = append(gauges, st)
	}
	for _, st := range udpStats {
		st.Timestamp = now
		st.Tags = append(st.Tags, hostTags...)
		gauges = append(gauges, st)
	}
	return counters, gauges, nil
}
