package network

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	selfExported "github.com/JulienBalestra/monitoring/pkg/collector/network/exported"
	"github.com/JulienBalestra/monitoring/pkg/tagger"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorConntrackName = "network-conntrack"

	conntrackPath = "/proc/net/ip_conntrack"
)

type Conntrack struct {
	conf     *collector.Config
	measures *metrics.Measures

	conntrackPath string

	tagLease, tagDevice *tagger.Tag
	unrepliedBytes      []byte
}

func NewConntrack(conf *collector.Config) collector.Collector {
	return newConntrack(conf)
}

func newConntrack(conf *collector.Config) *Conntrack {
	return &Conntrack{
		conf:           conf,
		measures:       metrics.NewMeasures(conf.SeriesCh),
		conntrackPath:  conntrackPath,
		tagLease:       tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),
		tagDevice:      tagger.NewTagUnsafe(selfExported.DeviceKey, tagger.MissingTagValue),
		unrepliedBytes: []byte("[UNREPLIED]"),
	}
}

func (c *Conntrack) Config() *collector.Config {
	return c.conf
}

func (c *Conntrack) IsDaemon() bool { return false }

func (c *Conntrack) Name() string {
	return CollectorConntrackName
}

func getPortRange(dstPort string) (string, error) {
	port, err := strconv.Atoi(dstPort)
	if err != nil {
		return "", err
	}
	if port < 1024 {
		return dstPort, nil
	}
	if port < 8191 {
		return "1024-8191", nil
	}
	if port < 49151 {
		return "8191-49151", nil
	}
	return "49152-65535", nil
}

type conntrackRecord struct {
	sourceIpAddress      string
	destinationPortRange string

	sBytes   float64
	dBytes   float64
	sPackets float64
	dPackets float64

	protocol string
	state    string
}

func (c *Conntrack) parseFields(stats map[string]*conntrackRecord, line []byte) error {
	var err error
	var protocol, state, dstPortRange string
	var srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex int

	// tcp      6 3 CLOSE
	//              ^
	index := bytes.IndexFunc(line[9:], func(r rune) bool {
		return (r >= 'A' && r <= 'Z') || r == 's'
	})
	fields := bytes.Fields(line[index+9:])
	switch line[0] {
	case 't':
		protocol = "tcp"
		if bytes.Equal(fields[7], c.unrepliedBytes) {
			state = "unreplied"
			srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex = 1, 5, 6, 12, 13
		} else {
			state = "replied"
			srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex = 1, 5, 6, 11, 12
		}
		dstPortRange, err = getPortRange(string(fields[4][6:]))
		if err != nil {
			return err
		}
	case 'u':
		protocol = "udp"
		if bytes.Equal(fields[6], c.unrepliedBytes) {
			state = "unreplied"
			srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex = 0, 4, 5, 11, 12
		} else {
			state = "replied"
			srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex = 0, 4, 5, 10, 11
		}
		s := string(fields[3][6:])
		dstPortRange, err = getPortRange(s)
		if err != nil {
			return err
		}
	case 'i':
		protocol = "icmp"
		//      type=8
		//           ^
		state = string(fields[2][5:])
		dstPortRange = "-1"
		srcIpIndex, sPacketsIndex, sBytesIndex, dPacketIndex, dBytesIndex = 0, 5, 6, 12, 13
	}
	//                       src=127.0.0.1
	//                           ^
	srcIp := string(fields[srcIpIndex][4:])
	key := protocol + dstPortRange + state + srcIp
	conn, ok := stats[key]
	if !ok {
		conn = &conntrackRecord{
			sourceIpAddress:      srcIp,
			destinationPortRange: dstPortRange,
			state:                state,
			protocol:             protocol,
		}
	}
	//                                         packets=119
	//                                                 ^
	sPackets, err := strconv.ParseFloat(string(fields[sPacketsIndex][8:]), 10)
	if err != nil {
		return err
	}
	//                                       bytes=7180
	//                                             ^
	sBytes, err := strconv.ParseFloat(string(fields[sBytesIndex][6:]), 10)
	if err != nil {
		return err
	}
	dPackets, err := strconv.ParseFloat(string(fields[dPacketIndex][8:]), 10)
	if err != nil {
		return err
	}
	dBytes, err := strconv.ParseFloat(string(fields[dBytesIndex][6:]), 10)
	if err != nil {
		return err
	}
	conn.sPackets += sPackets
	conn.sBytes += sBytes
	conn.dPackets += dPackets
	conn.dBytes += dBytes
	stats[key] = conn
	return nil
}

func (c *Conntrack) Collect(_ context.Context) error {
	file, err := os.Open(c.conntrackPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	stats := make(map[string]*conntrackRecord)
	for {
		// TODO improve this reader
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		err = c.parseFields(stats, line)
		if err != nil {
			log.Printf("failed to parse conntrack record: %s %v", string(line), err)
		}
	}
	hostTags := c.conf.Tagger.GetUnstable(c.conf.Host)
	now := time.Now()
	for _, record := range stats {
		tags := append(hostTags,
			c.conf.Tagger.GetUnstableWithDefault(record.sourceIpAddress,
				c.tagLease,
				c.tagDevice,
			)...)
		tags = append(tags,
			"protocol:"+record.protocol,
			"state:"+record.state,
			"dport:"+record.destinationPortRange,
			"ip:"+record.sourceIpAddress,
		)
		// TX: src client --> NAT --> dst server
		_ = c.measures.Count(&metrics.Sample{
			Name:      "network.conntrack.tx_packets",
			Value:     record.sPackets,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
		_ = c.measures.Count(&metrics.Sample{
			Name:      "network.conntrack.tx_bytes",
			Value:     record.sBytes,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})

		// RX: src client <-- NAT <-- dst server
		_ = c.measures.Count(&metrics.Sample{
			Name:      "network.conntrack.rx_packets",
			Value:     record.dPackets,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
		_ = c.measures.Count(&metrics.Sample{
			Name:      "network.conntrack.rx_bytes",
			Value:     record.dBytes,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		})
	}
	return nil
}
