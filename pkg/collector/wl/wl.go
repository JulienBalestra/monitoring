package wl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/macvendor"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	exportedTags "github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorName = "wl"

	optionWLBinary     = "wl-exe"
	optionWirelessPath = "proc-net-wireless-path"

	wirelessMetricPrefix = "network.wireless."
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	commaByte   []byte
	endlineByte []byte
	spaceByte   []byte
	semiCoByte  []byte

	wlCommands         []*wlCommand
	wlCommandsToUpdate time.Time

	defaultLeaseTag *tagger.Tag
}

type wlCommand struct {
	device string
	ssid   string
}

func NewWL(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(newWL(conf))
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
	return map[string]string{
		optionWLBinary:     "/usr/sbin/wl",
		optionWirelessPath: "/proc/net/wireless",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 15
}

func newWL(conf *collector.Config) *Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		// alloc once
		commaByte:       []byte{'"'},
		endlineByte:     []byte{'\n'},
		spaceByte:       []byte{' '},
		semiCoByte:      []byte{':'},
		defaultLeaseTag: tagger.NewTagUnsafe(exportedTags.LeaseKey, tagger.MissingTagValue),
	}
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) getSSID(b []byte) (string, error) {
	start := bytes.Index(b, c.commaByte)
	if start == -1 {
		return "", errors.New("invalid wl output")
	}
	end := bytes.Index(b[start+1:], c.commaByte)
	if end == -1 {
		return "", errors.New("invalid wl output")
	}
	return string(b[start+1 : start+end+1]), nil
}

func (c *Collector) getWLCommands(ctx context.Context) ([]*wlCommand, error) {
	var wlCommands []*wlCommand

	wlBinaryFile, ok := c.conf.Options[optionWLBinary]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionWLBinary))
		return wlCommands, errors.New("missing option " + optionWLBinary)
	}

	procNetWirelessPath, ok := c.conf.Options[optionWirelessPath]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionWirelessPath))
		return wlCommands, errors.New("missing option " + optionWirelessPath)
	}

	now := time.Now()
	if now.Before(c.wlCommandsToUpdate) && c.wlCommands != nil {
		return c.wlCommands, nil
	}
	f, err := os.Open(procNetWirelessPath)
	if err != nil {
		return c.wlCommands, err
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, 256)
	var devices []string
	l := 0
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		if l < 2 {
			l++
			continue
		}
		index := bytes.Index(line[2:], c.semiCoByte)
		if index == -1 {
			continue
		}
		devices = append(devices, string(line[2:index+2]))
	}

	for _, device := range devices {
		b, err := exec.CommandContext(ctx, wlBinaryFile, "-i", device, "status").CombinedOutput()
		if err != nil {
			continue
		}
		ssid, err := c.getSSID(b)
		if err != nil {
			zap.L().Error("failed to get the SSID",
				zap.String("device", device),
				zap.Error(err),
			)
			continue
		}
		wlCommands = append(wlCommands,
			&wlCommand{
				device: device,
				ssid:   ssid,
			},
		)
	}

	// cache this for the added duration
	c.wlCommands = wlCommands
	c.wlCommandsToUpdate = now.Add(time.Minute * 10)
	return c.wlCommands, nil
}

func (c *Collector) getMacs(b []byte) []string {
	var macs []string
	for _, line := range bytes.Split(b, c.endlineByte) {
		index := bytes.Index(line, c.spaceByte)
		if index == -1 {
			continue
		}
		macs = append(macs, string(line[index+1:]))
	}
	return macs
}

func (c *Collector) Collect(ctx context.Context) error {
	wlBinary, ok := c.conf.Options[optionWLBinary]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionWLBinary))
		return errors.New("missing option " + optionWLBinary)
	}

	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	wlCommands, err := c.getWLCommands(ctx)
	if err != nil {
		return err
	}
	hostTags := c.Tags()
	for _, command := range wlCommands {
		b, err := exec.CommandContext(ctx, wlBinary, "-i", command.device, "assoclist").CombinedOutput()
		if err != nil {
			zap.L().Error("failed to run command", zap.Error(err),
				zap.String("device", command.device),
			)
			continue
		}
		for _, mac := range c.getMacs(b) {
			macAddress := macvendor.NormaliseMacAddress(mac)
			vendor := macvendor.GetVendorWithMacOrUnknown(macAddress)
			b, err := exec.CommandContext(ctx, wlBinary, "-i", command.device, "rssi", mac).CombinedOutput()
			if err != nil {
				zap.L().Error("failed to run command", zap.Error(err),
					zap.String("device", command.device),
					zap.String("mac", mac),
					zap.String("vendor", vendor),
				)
				continue
			}
			rssi, err := strconv.ParseFloat(string(b[:len(b)-1]), 10)
			if err != nil {
				zap.L().Error("failed to parse rssi", zap.Error(err),
					zap.String("device", command.device),
					zap.String("mac", mac),
					zap.String("vendor", vendor),
				)
				continue
			}
			if rssi >= 0. {
				zap.L().Warn("invalid rssi", zap.Float64("rssi", rssi),
					zap.String("device", command.device),
					zap.String("mac", mac),
					zap.String("vendor", vendor),
				)
				continue
			}
			deviceTag := tagger.NewTagUnsafe("device", command.device)
			ssidTag := tagger.NewTagUnsafe("ssid", command.ssid)
			vendorTag := tagger.NewTagUnsafe("vendor", vendor)
			c.conf.Tagger.Update(macAddress, deviceTag, ssidTag, vendorTag)

			tags := append(hostTags, c.conf.Tagger.GetUnstableWithDefault(macAddress, c.defaultLeaseTag)...)
			tags = append(tags, "mac:"+macAddress)
			s := &metrics.Sample{
				Name:  wirelessMetricPrefix + "rssi.dbm",
				Value: rssi,
				Time:  time.Now(),
				Host:  c.conf.Host,
				Tags:  tags,
			}
			c.measures.GaugeDeviation(s, c.conf.CollectInterval*c.conf.CollectInterval)
		}
	}

	c.measures.Purge()
	return nil
}
