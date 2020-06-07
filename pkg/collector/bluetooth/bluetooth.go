package bluetooth

import (
	"context"
	"github.com/JulienBalestra/monitoring/pkg/collector/bluetooth/exported"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/fnv"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/godbus/dbus"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"go.uber.org/zap"
)

const (
	CollectorLoadName = "bluetooth"
)

type Bluetooth struct {
	conf     *collector.Config
	measures *metrics.Measures
	replacer *strings.Replacer
}

func NewBluetooth(conf *collector.Config) collector.Collector {
	return &Bluetooth{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
		replacer: strings.NewReplacer(
			":", "-",
			" ", "-",
		),
	}
}

func (c *Bluetooth) Config() *collector.Config {
	return c.conf
}

func (c *Bluetooth) IsDaemon() bool { return true }

func (c *Bluetooth) Name() string {
	return CollectorLoadName
}

func (c *Bluetooth) Collect(ctx context.Context) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	ag := agent.NewSimpleAgent()
	defer ag.Cancel()

	err = agent.ExposeAgent(conn, ag, agent.CapKeyboardDisplay, true)
	if err != nil {
		return err
	}
	defer agent.RemoveAgent(ag)

	a, err := adapter.GetDefaultAdapter()
	if err != nil {
		return err
	}
	defer a.Close()

	err = a.FlushDevices()
	if err != nil {
		return err
	}

	err = a.StartDiscovery()
	if err != nil {
		return err
	}
	defer a.Close()

	// TODO revamp this
	err = a.Client().Connect()
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			devices, err := a.GetDevices()
			if err != nil {
				return err
			}
			for _, device := range devices {
				var newTags []*tagger.Tag
				if device.Properties.AddressType != "" {
					newTags = append(newTags, tagger.NewTagUnsafe("address-type", c.replacer.Replace(device.Properties.AddressType)))
				}
				alias := c.replacer.Replace(device.Properties.Alias)
				if alias != "" {
					newTags = append(newTags, tagger.NewTagUnsafe(exported.AliasKey, alias))
				}
				name := c.replacer.Replace(device.Properties.Name)
				if name != "" {
					newTags = append(newTags, tagger.NewTagUnsafe(exported.NameKey, name))
				}

				sort.Strings(device.Properties.UUIDs)
				h := fnv.NewHash()
				for _, elt := range device.Properties.UUIDs {
					h = fnv.AddString(h, elt)
				}

				mac := c.replacer.Replace(device.Properties.Address)
				c.conf.Tagger.Update(mac, newTags...)
				tags := append(c.conf.Tagger.GetUnstableWithDefault(mac,
					tagger.NewTagUnsafe(exported.NameKey, "unknown"),
					tagger.NewTagUnsafe(exported.AliasKey, "unknown"),
				),
					"mac:"+mac,

					"uuids-hash:"+strconv.FormatUint(h, 10),

					"connected:"+strconv.FormatBool(device.Properties.Connected),
					"trusted:"+strconv.FormatBool(device.Properties.Trusted),
					"blocked:"+strconv.FormatBool(device.Properties.Blocked),
					"paired:"+strconv.FormatBool(device.Properties.Paired),
				)
				zctx := zap.L().With(
					zap.String("name", name),
					zap.String("mac", mac),
					zap.String("alias", alias),

					zap.String("addressType", device.Properties.AddressType),
					zap.Int16("rssi", device.Properties.RSSI),

					zap.Bool("connected", device.Properties.Connected),
					zap.Bool("trusted", device.Properties.Trusted),
					zap.Bool("blocked", device.Properties.Blocked),
					zap.Bool("paired", device.Properties.Paired),
				)
				zctx.Debug("found device")

				c.measures.GaugeDeviation(&metrics.Sample{
					Name:      "bluetooth.rssi.dbm",
					Value:     float64(device.Properties.RSSI),
					Timestamp: time.Now(),
					Host:      c.conf.Host,
					Tags:      tags,
				}, c.conf.CollectInterval*3)
			}
			wCtx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
			<-wCtx.Done()
			cancel()
		}
	}
}
