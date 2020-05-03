package wl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	exportedTags "github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
)

const (
	CollectorWLName = "wl"
	wlBinaryPath    = "/usr/sbin/wl"

	wirelessMetricPrefix = "network.wireless."
)

type WL struct {
	conf     *collector.Config
	measures *metrics.Measures

	commaByte   []byte
	endlineByte []byte
	spaceByte   []byte
	semiCoByte  []byte

	wlCommands         []*wlCommand
	wlCommandsToUpdate time.Time

	leaseTag *tagger.Tag

	deviationCache map[string]*metrics.Sample
}

type wlCommand struct {
	device string
	ssid   string
}

func NewWL(conf *collector.Config) collector.Collector {
	return newWL(conf)
}

func newWL(conf *collector.Config) *WL {
	return &WL{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),

		// alloc once
		commaByte:      []byte{'"'},
		endlineByte:    []byte{'\n'},
		spaceByte:      []byte{' '},
		semiCoByte:     []byte{':'},
		deviationCache: make(map[string]*metrics.Sample),

		leaseTag: tagger.NewTagUnsafe(exportedTags.LeaseKey, tagger.MissingTagValue),
	}
}

func (c *WL) Config() *collector.Config {
	return c.conf
}

func (c *WL) IsDaemon() bool { return false }

func (c *WL) Name() string {
	return CollectorWLName
}

func (c *WL) getSSID(b []byte) (string, error) {
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

func (c *WL) getWLCommands(ctx context.Context) ([]*wlCommand, error) {
	var wlCommands []*wlCommand
	now := time.Now()
	if now.Before(c.wlCommandsToUpdate) && c.wlCommands != nil {
		return c.wlCommands, nil
	}
	f, err := os.Open("/proc/net/wireless")
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
		b, err := exec.CommandContext(ctx, wlBinaryPath, "-i", device, "status").CombinedOutput()
		if err != nil {
			continue
		}
		ssid, err := c.getSSID(b)
		if err != nil {
			log.Printf("failed to get the SSID for %q %v", device, err)
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

func (c *WL) getMacs(b []byte) []string {
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

func (c *WL) Collect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	wlCommands, err := c.getWLCommands(ctx)
	if err != nil {
		return err
	}
	hostTags := c.conf.Tagger.GetUnstable(c.conf.Host)
	for _, command := range wlCommands {
		b, err := exec.CommandContext(ctx, wlBinaryPath, "-i", command.device, "assoclist").CombinedOutput()
		if err != nil {
			log.Printf("failed to run command: %v", err)
			continue
		}
		for _, mac := range c.getMacs(b) {
			b, err := exec.CommandContext(ctx, wlBinaryPath, "-i", command.device, "rssi", mac).CombinedOutput()
			if err != nil {
				log.Printf("failed to run command: %v", err)
				continue
			}
			rssi, err := strconv.ParseFloat(string(b[:len(b)-1]), 10)
			if err != nil {
				log.Printf("failed to parse rssi: %v", err)
				continue
			}
			macAddress := strings.ToLower(strings.ReplaceAll(mac, ":", "-"))
			deviceTag := tagger.NewTagUnsafe("device", command.device)
			ssidTag := tagger.NewTagUnsafe("ssid", command.ssid)
			c.conf.Tagger.Update(macAddress, deviceTag, ssidTag)

			tags := append(hostTags, c.conf.Tagger.GetUnstableWithDefault(macAddress, c.leaseTag)...)
			tags = append(tags, "mac:"+macAddress)
			s := &metrics.Sample{
				Name:      wirelessMetricPrefix + "rssi.dbm",
				Value:     rssi,
				Timestamp: time.Now(),
				Host:      c.conf.Host,
				Tags:      tags,
			}
			c.measures.GaugeDeviation(s, c.conf.CollectInterval*3)
		}
	}
	return nil
}
