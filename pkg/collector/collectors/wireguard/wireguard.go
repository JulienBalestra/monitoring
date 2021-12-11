package wireguard

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	stun "github.com/JulienBalestra/wireguard-stun/pkg/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
)

const (
	CollectorName = "wireguard"

	wireguardMetricPrefix = "wireguard."

	none = "none"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewWireguard(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(&Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
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
	return map[string]string{}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 10
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func getAllowedIPsTag(n []net.IPNet) string {
	l := len(n)
	if l == 0 {
		return none
	}
	if l == 1 {
		return n[0].String()
	}

	allowedIps := make([]string, 0, l)
	for _, i := range n {
		s := i.String()
		allowedIps = append(allowedIps, s)
	}
	sort.Strings(allowedIps)
	return strings.Join(allowedIps, ",")
}

func (c *Collector) setStatus(now time.Time, tags []string, isActive bool) {
	active, inactive := 0., 1.
	if isActive {
		// swap
		active, inactive = 1., 0.
	}
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  wireguardMetricPrefix + "active",
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
		Value: active,
	},
		c.conf.CollectInterval,
	)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  wireguardMetricPrefix + "inactive",
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
		Value: inactive,
	},
		c.conf.CollectInterval,
	)
}

func (c *Collector) Collect(_ context.Context) error {
	wgc, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgc.Close()

	devices, err := wgc.Devices()
	if err != nil {
		return err
	}

	now := time.Now()
	hostTags := c.Tags()
	for _, device := range devices {
		deviceTag := tagger.NewTagUnsafe("device", device.Name)
		for _, peer := range device.Peers {
			peerSHA := stun.NewPeer(&peer)

			age := now.Sub(peerSHA.LastHandshakeTime)
			active := peer.Endpoint != nil && age < time.Second*210
			activeTag := tagger.NewTagUnsafe("wg-active", strconv.FormatBool(active))

			pubKeySha1Tag := tagger.NewTagUnsafe("pub-key-sha1", peerSHA.PublicKeySha1)
			pubKeySha1TruncTag := tagger.NewTagUnsafe("pub-key-sha1-7", peerSHA.PublicKeyShortSha1)
			allowedIpsTag := tagger.NewTagUnsafe("allowed-ips", getAllowedIPsTag(peerSHA.AllowedIPs))

			if peer.Endpoint == nil {
				endpointTag := tagger.NewTagUnsafe("endpoint", none)
				ipTag := tagger.NewTagUnsafe("ip", none)
				portTag := tagger.NewTagUnsafe("port", none)
				c.conf.Tagger.Update(peerSHA.PublicKey.String(),
					pubKeySha1Tag,
					pubKeySha1TruncTag,
					allowedIpsTag,
					activeTag,
					deviceTag,
					endpointTag,
					ipTag,
					portTag,
				)
				c.conf.Tagger.Update(peerSHA.PublicKeySha1,
					pubKeySha1TruncTag,
					allowedIpsTag,
					activeTag,
					deviceTag,
					endpointTag,
					ipTag,
					portTag,
				)
				tags := append(hostTags, c.conf.Tagger.GetUnstable(peerSHA.PublicKey.String())...)
				c.setStatus(now, tags, false)
				continue
			}

			endpointTag := tagger.NewTagUnsafe("endpoint", peerSHA.Endpoint.String())
			ipTag := tagger.NewTagUnsafe("ip", peerSHA.Endpoint.IP.String())
			portTag := tagger.NewTagUnsafe("port", strconv.Itoa(peerSHA.Endpoint.Port))
			c.conf.Tagger.Update(peerSHA.PublicKey.String(),
				pubKeySha1Tag,
				pubKeySha1TruncTag,
				allowedIpsTag,
				activeTag,
				deviceTag,
				endpointTag,
				ipTag,
				portTag,
			)
			c.conf.Tagger.Update(peerSHA.PublicKeySha1,
				pubKeySha1TruncTag,
				allowedIpsTag,
				activeTag,
				deviceTag,
				endpointTag,
				ipTag,
				portTag,
			)
			tags := append(hostTags, c.conf.Tagger.GetUnstable(peerSHA.PublicKey.String())...)
			c.setStatus(now, tags, active)
			_ = c.measures.CountWithNegativeReset(&metrics.Sample{
				Name:  wireguardMetricPrefix + "transfer.received",
				Value: float64(peerSHA.ReceiveBytes),
				Time:  now,
				Host:  c.conf.Host,
				Tags:  tags,
			})
			_ = c.measures.CountWithNegativeReset(&metrics.Sample{
				Name:  wireguardMetricPrefix + "transfer.sent",
				Value: float64(peerSHA.TransmitBytes),
				Time:  now,
				Host:  c.conf.Host,
				Tags:  tags,
			})
			c.measures.Gauge(&metrics.Sample{
				Name:  wireguardMetricPrefix + "handshake.age",
				Value: age.Seconds(),
				Time:  now,
				Host:  c.conf.Host,
				Tags:  tags,
			})
		}
	}
	c.measures.Purge()
	return nil
}
