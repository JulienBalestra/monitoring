package network

import (
	"context"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/conntrack"

	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	selfExported "github.com/JulienBalestra/monitoring/pkg/collector/network/exported"
	"github.com/JulienBalestra/monitoring/pkg/tagger"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorConntrackName = "network-conntrack"

	conntrackPath = "/proc/net/ip_conntrack"

	deadlineTolerationDuration = time.Second * 5
	maxAgeConntrackEntries     = time.Hour
)

type Conntrack struct {
	conf     *collector.Config
	measures *metrics.Measures

	conntrackPath string

	tagLease, tagDevice *tagger.Tag
}

func NewConntrack(conf *collector.Config) collector.Collector {
	return newConntrack(conf)
}

func newConntrack(conf *collector.Config) *Conntrack {
	return &Conntrack{
		conf:          conf,
		measures:      metrics.NewMeasuresWithMaxAge(conf.SeriesCh, maxAgeConntrackEntries),
		conntrackPath: conntrackPath,
		tagLease:      tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),
		tagDevice:     tagger.NewTagUnsafe(selfExported.DeviceKey, tagger.MissingTagValue),
	}
}

func (c *Conntrack) Config() *collector.Config {
	return c.conf
}

func (c *Conntrack) IsDaemon() bool { return true }

func (c *Conntrack) Name() string {
	return CollectorConntrackName
}

func getPortRange(port int) string {
	if port < 1024 {
		return strconv.Itoa(port)
	}
	if port < 8191 {
		return "1024-8191"
	}
	if port < 49151 {
		return "8191-49151"
	}
	return "49152-65535"
}

type aggregation struct {
	toBytes              float64
	fromBytes            float64
	protocol             string
	destinationPortRange string
	sourceIP             string
	Timestamp            time.Time
}

func (c *Conntrack) aggregationToSamples(now time.Time, aggr *aggregation) (*metrics.Sample, *metrics.Sample) {
	tags := append(c.conf.Tagger.GetUnstable(c.conf.Host),
		c.conf.Tagger.GetUnstableWithDefault(aggr.sourceIP,
			c.tagLease,
			c.tagDevice,
		)...)
	tags = append(tags,
		"protocol:"+aggr.protocol,
		"dport:"+aggr.destinationPortRange,
		"ip:"+aggr.sourceIP,
	)
	return &metrics.Sample{
			Name:      "network.conntrack.rx_bytes",
			Value:     aggr.toBytes,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		},
		&metrics.Sample{
			Name:      "network.conntrack.tx_bytes",
			Value:     aggr.fromBytes,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		}
}

func (c *Conntrack) Collect(ctx context.Context) error {
	after := time.After(0)

	records := make(map[uint64]*conntrack.Record)
	aggregations := make(map[string]*aggregation)

	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			now := time.Now()
			for _, aggr := range aggregations {
				tx, rx := c.aggregationToSamples(now, aggr)
				_, _ = c.measures.Count(tx), c.measures.Count(rx)
			}
			c.measures.Purge()
			for h, record := range records {
				if time.Since(record.Deadline) > deadlineTolerationDuration {
					delete(records, h)
				}
			}
			for key, value := range aggregations {
				if time.Since(value.Timestamp) > maxAgeConntrackEntries {
					delete(aggregations, key)
				}
			}

		case <-after:
			newRecords, closestDeadline, err := conntrack.GetConntrackRecords(conntrackPath)
			if err != nil {
				continue
			}
			closestDeadlineIn := closestDeadline.Sub(time.Now())
			if closestDeadlineIn < deadlineTolerationDuration {
				closestDeadlineIn = deadlineTolerationDuration
			}
			after = time.After(closestDeadlineIn)
			now := time.Now()

			for h, newRecord := range newRecords {
				switch newRecord.Protocol {
				case conntrack.ProtocolTCP:
					if newRecord.State != conntrack.StateEstablished {
						continue
					}
				default:
					if newRecord.State != conntrack.StateReplied {
						continue
					}
				}
				portRange := getPortRange(newRecord.From.Quad.DestinationPort)
				aKey := newRecord.Protocol + newRecord.From.Quad.Source + portRange

				aggr, ok := aggregations[aKey]
				if !ok {
					aggr = &aggregation{
						protocol:             newRecord.Protocol,
						destinationPortRange: portRange,
						sourceIP:             newRecord.From.Quad.Source,
						toBytes:              newRecord.To.Bytes,
						fromBytes:            newRecord.From.Bytes,
						Timestamp:            now,
					}
					aggregations[aKey] = aggr
					records[h] = newRecord
					continue
				}
				existing, ok := records[h]
				if ok && time.Since(existing.Deadline) < deadlineTolerationDuration {
					aggr.toBytes += newRecord.To.Bytes - existing.To.Bytes
					aggr.fromBytes += newRecord.From.Bytes - existing.From.Bytes
				} else {
					aggr.toBytes += newRecord.To.Bytes
					aggr.fromBytes += newRecord.From.Bytes
				}
				aggr.Timestamp = now
				records[h] = newRecord
			}
		}
	}
}
