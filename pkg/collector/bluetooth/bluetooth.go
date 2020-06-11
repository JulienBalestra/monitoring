package bluetooth

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/bluetooth/exported"
	"github.com/JulienBalestra/monitoring/pkg/fnv"
	"github.com/JulienBalestra/monitoring/pkg/mac"
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
		zap.L().Error("failed to create dbus connection", zap.Error(err))
		return err
	}
	defer conn.Close()

	ag := agent.NewSimpleAgent()
	zctx := zap.L().With(
		zap.String("agentCapability", agent.CapDisplayOnly),
		zap.String("agentInterface", ag.Interface()),
	)

	err = agent.ExposeAgent(conn, ag, agent.CapDisplayOnly, false)
	if err != nil {
		zctx.Error("failed to expose agent", zap.Error(err))
		return err
	}
	defer agent.RemoveAgent(ag)

	a, err := adapter.GetDefaultAdapter()
	if err != nil {
		zctx.Error("failed to get default adapter", zap.Error(err))
		return err
	}
	defer a.Close()

	err = a.FlushDevices()
	if err != nil {
		zap.L().Error("failed to flush devices", zap.Error(err))
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		default:
		}
		zctx.Info("starting discovery")
		err = a.StartDiscovery()
		if err == nil {
			zctx.Info("started discovery")
			break
		}
		zctx.Warn("failed to start discovery", zap.Error(err))
		wCtx, cancel := context.WithTimeout(ctx, time.Second)
		<-wCtx.Done()
		cancel()
	}
	defer a.Close()

	seenDevices := make(map[string]map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			devices, err := a.GetDevices()
			if err != nil {
				zap.L().Error("failed to get devices", zap.Error(err))
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

				macAddress := c.replacer.Replace(device.Properties.Address)
				vendor, ok := mac.GetVendor(macAddress)
				if ok {
					newTags = append(newTags, tagger.NewTagUnsafe(exported.MacVendorKey, vendor))
				}
				c.conf.Tagger.Update(macAddress, newTags...)
				tags := append(c.conf.Tagger.GetUnstableWithDefault(macAddress,
					tagger.NewTagUnsafe(exported.NameKey, "unknown"),
					tagger.NewTagUnsafe(exported.AliasKey, "unknown"),
					tagger.NewTagUnsafe(exported.MacVendorKey, "unknown"),
				),
					"mac:"+macAddress,

					"uuids-hash:"+strconv.FormatUint(h, 10),

					"connected:"+strconv.FormatBool(device.Properties.Connected),
					"trusted:"+strconv.FormatBool(device.Properties.Trusted),
					"blocked:"+strconv.FormatBool(device.Properties.Blocked),
					"paired:"+strconv.FormatBool(device.Properties.Paired),
				)
				dzctx := zctx.With(
					zap.String("name", name),
					zap.String("mac", macAddress),
					zap.String("alias", alias),
					zap.String("vendor", vendor),

					zap.String("addressType", device.Properties.AddressType),
					zap.Int16("rssi", device.Properties.RSSI),

					zap.Bool("connected", device.Properties.Connected),
					zap.Bool("trusted", device.Properties.Trusted),
					zap.Bool("blocked", device.Properties.Blocked),
					zap.Bool("paired", device.Properties.Paired),
				)
				dzctx.Debug("found device")

				c.measures.GaugeDeviation(&metrics.Sample{
					Name:      "bluetooth.rssi.dbm",
					Value:     float64(device.Properties.RSSI),
					Timestamp: time.Now(),
					Host:      c.conf.Host,
					Tags:      tags,
				}, c.conf.CollectInterval*3)

				err = a.RemoveDevice(device.Path())
				if err != nil {
					dzctx.Error("failed to remove device", zap.Error(err))
				}
				if vendor == "" {
					continue
				}
				_, ok = seenDevices[vendor]
				if !ok {
					seenDevices[vendor] = make(map[string]struct{})
				}
				seenDevices[vendor][macAddress] = struct{}{}
			}

			for vendor := range seenDevices {
				nb := len(seenDevices[vendor])
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:      "bluetooth.devices",
					Value:     float64(nb),
					Timestamp: time.Now(),
					Host:      c.conf.Host,
					Tags: []string{
						"vendor:" + vendor,
					},
				}, c.conf.CollectInterval*6)
			}
			wCtx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
			<-wCtx.Done()
			cancel()
		}
	}
}
