package network

import (
	"bufio"
	"context"
	"errors"
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
	srcPrefix     = "src="
)

type Conntrack struct {
	conf *collector.Config
}

func NewConntrack(conf *collector.Config) collector.Collector {
	return &Conntrack{
		conf: conf,
	}
}

func (c *Conntrack) Config() *collector.Config {
	return c.conf
}

func (c *Conntrack) IsDaemon() bool { return false }

func (c *Conntrack) Name() string {
	return CollectorConntrackName
}

func getPortTag(portField string) (string, error) {
	const portTagPrefix = "dst_port:"
	dstPort := strings.TrimPrefix(portField, "dport=")
	port, err := strconv.Atoi(dstPort)
	if err != nil {
		return "", err
	}
	if port < 1024 {
		return portTagPrefix + dstPort, nil
	}
	if port < 8191 {
		return portTagPrefix + "1024-8191", nil
	}
	if port < 49151 {
		return portTagPrefix + "8191-49151", nil
	}
	return portTagPrefix + "49152-65535", nil
}

func (c *Conntrack) parseTCPFields(fields []string, tcpStats map[string]*datadog.Metric) error {
	if len(fields) < 8 {
		return errors.New("incorrect tcp fields")
	}
	state, srcIp, dstPort := fields[3], fields[4], fields[7]
	portTag, err := getPortTag(dstPort)
	if err != nil {
		return err
	}
	srcIp = strings.TrimPrefix(srcIp, srcPrefix)
	mapKey := srcIp + state + dstPort
	st, ok := tcpStats[mapKey]
	if !ok {
		st = &datadog.Metric{
			Name: "network.conntrack.tcp",
			Host: c.conf.Host,
			Tags: append(c.conf.Tagger.GetWithDefault(srcIp,
				tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),
				tagger.NewTagUnsafe(selfExported.DeviceKey, tagger.MissingTagValue),
			), "state:"+state, "src_ip:"+srcIp, portTag),
		}
		tcpStats[mapKey] = st
	}
	st.Value++
	return nil
}

func (c *Conntrack) parseUDPFields(fields []string, udpStats map[string]*datadog.Metric) error {
	if len(fields) < 7 {
		return errors.New("incorrect udp fields")
	}
	srcIp, dstPort := fields[3], fields[6]
	portTag, err := getPortTag(dstPort)
	if err != nil {
		return err
	}
	srcIp = strings.TrimPrefix(srcIp, srcPrefix)
	mapKey := srcIp + dstPort
	st, ok := udpStats[mapKey]
	if !ok {
		st = &datadog.Metric{
			Name: "network.conntrack.udp",
			Host: c.conf.Host,
			Tags: append(c.conf.Tagger.GetWithDefault(srcIp,
				tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),
				tagger.NewTagUnsafe(selfExported.DeviceKey, tagger.MissingTagValue),
			), "src_ip:"+srcIp, portTag),
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
		// TODO improve this reader
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return counters, gauges, err
		}
		fields := strings.Fields(string(line))
		switch fields[0] {
		case "tcp":
			_ = c.parseTCPFields(fields, tcpStats)
		case "udp":
			_ = c.parseUDPFields(fields, udpStats)
		}
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
