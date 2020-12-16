package wireguard

import (
	"context"
	"encoding/base32"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"golang.zx2c4.com/wireguard/wgctrl"
)

const (
	CollectorWireguardName = "wireguard"

	wireguardMetricPrefix = "wireguard."
)

type Wireguard struct {
	conf     *collector.Config
	measures *metrics.Measures
}

func NewWireguard(conf *collector.Config) collector.Collector {
	return &Wireguard{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Wireguard) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Wireguard) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Wireguard) Config() *collector.Config {
	return c.conf
}

func (c *Wireguard) IsDaemon() bool { return false }

func (c *Wireguard) Name() string {
	return CollectorWireguardName
}

func (c *Wireguard) Collect(_ context.Context) error {
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
	for _, device := range devices {
		for _, peer := range device.Peers {
			peerPublicKeyB32 := base32.HexEncoding.EncodeToString([]byte(peer.PublicKey.String()))
			c.conf.Tagger.Update(peer.PublicKey.String(),
				tagger.NewTagUnsafe("endpoint", peer.Endpoint.String()),
				tagger.NewTagUnsafe("ip", peer.Endpoint.IP.String()),
				tagger.NewTagUnsafe("port", strconv.Itoa(peer.Endpoint.Port)),
				tagger.NewTagUnsafe("device", device.Name),
				tagger.NewTagUnsafe("pub-b32hex", peerPublicKeyB32),
			)
			tags := c.conf.Tagger.GetUnstable(peer.PublicKey.String())
			_ = c.measures.Count(&metrics.Sample{
				Name:      wireguardMetricPrefix + "transfer.received",
				Value:     float64(peer.ReceiveBytes),
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
			_ = c.measures.Count(&metrics.Sample{
				Name:      wireguardMetricPrefix + "transfer.sent",
				Value:     float64(peer.TransmitBytes),
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
			c.measures.Gauge(&metrics.Sample{
				Name:      wireguardMetricPrefix + "handshake.age",
				Value:     now.Sub(peer.LastHandshakeTime).Seconds(),
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
		}
	}
	return nil
}
