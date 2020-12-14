package network

import (
	"context"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	selfExported "github.com/JulienBalestra/monitoring/pkg/collector/network/exported"
	"github.com/JulienBalestra/monitoring/pkg/conntrack"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorConntrackName = "network-conntrack"

	conntrackPath = "/proc/net/ip_conntrack"

	maxAgeConntrackEntries = time.Hour
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
		measures:      metrics.NewMeasuresWithMaxAge(conf.MetricsClient.ChanSeries, maxAgeConntrackEntries),
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
	count                float64
	protocol             string
	destinationPortRange string
	sourceIP             string
	state                string
}

func (c *Conntrack) aggregationToSamples(now time.Time, aggr *aggregation) *metrics.Sample {
	tags := append(c.conf.Tagger.GetUnstable(c.conf.Host),
		c.conf.Tagger.GetUnstableWithDefault(aggr.sourceIP,
			c.tagLease,
			c.tagDevice,
		)...)
	tags = append(tags,
		"protocol:"+aggr.protocol,
		"dport:"+aggr.destinationPortRange,
		"ip:"+aggr.sourceIP,
		"state:"+aggr.state,
	)
	return &metrics.Sample{
		Name:      "network.conntrack.entries",
		Value:     aggr.count,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      tags,
	}
}

func (c *Conntrack) Collect(ctx context.Context) error {
	after := time.After(0)

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
				_ = c.measures.GaugeDeviation(c.aggregationToSamples(now, aggr), c.conf.CollectInterval*3)
			}
			c.measures.Purge()
			aggregations = make(map[string]*aggregation)

		case <-after:
			newRecords, closestDeadline, err := conntrack.GetConntrackRecords(conntrackPath)
			if err != nil {
				zap.L().Error("failed to get conntrack records", zap.Error(err))
				continue
			}
			closestDeadlineIn := closestDeadline.Sub(time.Now())
			if closestDeadlineIn < c.conf.CollectInterval {
				closestDeadlineIn = c.conf.CollectInterval
			}
			after = time.After(closestDeadlineIn)

			for _, newRecord := range newRecords {
				portRange := getPortRange(newRecord.From.Quad.DestinationPort)
				aKey := newRecord.Protocol + newRecord.From.Quad.Source + portRange

				aggr, ok := aggregations[aKey]
				if !ok {
					aggr = &aggregation{
						protocol:             newRecord.Protocol,
						destinationPortRange: portRange,
						sourceIP:             newRecord.From.Quad.Source,
						state:                newRecord.State,
					}
					aggregations[aKey] = aggr
				}
				aggr.count++
			}
		}
	}
}
