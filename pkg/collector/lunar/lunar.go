package lunar

import (
	"context"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/godbus/dbus"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"go.uber.org/zap"
)

const (
	CollectorLoadName = "acaia-lunar"

	lunarUUID        = "00001820-0000-1000-8000-00805f9b34fb"
	lunarServiceUUID = "00002a80-0000-1000-8000-00805f9b34fb"
)

type Lunar struct {
	conf     *collector.Config
	measures *metrics.Measures

	sequenceID byte
}

func NewAcaia(conf *collector.Config) collector.Collector {
	return &Lunar{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Lunar) Config() *collector.Config {
	return c.conf
}

func (c *Lunar) IsDaemon() bool { return true }

func (c *Lunar) Name() string {
	return CollectorLoadName
}

func (c *Lunar) lunar(d *device.Device1) error {
	zap.L().Debug("pairing")
	err := d.Pair()
	if err != nil {
		zap.L().Error("failed to pair", zap.Error(err))
		return err
	}

	zap.L().Debug("connecting")
	err = d.Connect()
	if err != nil {
		zap.L().Error("failed to connect", zap.Error(err))
		return err
	}
	defer d.Disconnect()

	char, err := d.GetCharByUUID(lunarServiceUUID)
	if err != nil {
		zap.L().Error("failed to GetCharacteristics", zap.Error(err))
		return err
	}
	c.sequenceID++
	c.sequenceID &= 0xff
	// TODO write https://github.com/bpowers/btscale/blob/master/src/packet.ts#L80
	b := []byte{
		0xdf,
		0x78,
		5 + 0x5, // timer
		c.sequenceID,
		0,
		0x5 & 0xff,
	}
	err = char.WriteValue(b,
		make(map[string]interface{}),
	)
	if err != nil {
		zap.L().Error("failed to WriteValue", zap.Error(err))
		return err
	}
	zap.L().Info("msg", zap.ByteString("b", b))
	return nil
}

func (c *Lunar) Collect(ctx context.Context) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	ag := agent.NewSimpleAgent()
	defer ag.Cancel()

	err = agent.ExposeAgent(conn, ag, agent.CapKeyboardDisplay, false)
	if err != nil {
		return err
	}
	defer agent.RemoveAgent(ag)

	a, err := adapter.GetDefaultAdapter()
	if err != nil {
		return err
	}
	defer a.Close()

	err = a.StartDiscovery()
	if err != nil {
		return err
	}
	defer a.StopDiscovery()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			zap.L().Debug("finding devices")
			devices, err := a.GetDevices()
			if err != nil {
				return err
			}
			for _, d := range devices {
				for _, u := range d.Properties.UUIDs {
					if u != lunarUUID {
						continue
					}
					_ = c.lunar(d)
				}
			}
			wCtx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
			<-wCtx.Done()
			cancel()
		}
	}
}
