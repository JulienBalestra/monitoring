package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorShellyName = "shelly"
)

type Shelly struct {
	conf      *collector.Config
	measures  *metrics.Measures
	client    *http.Client
	shellyUrl string
}

type Status struct {
	WifiSTA struct {
		SSID string `json:"ssid"`
		IP   string `json:"ip"`
		RSSI int64  `json:"rssi"`
	} `json:"wifi_sta"`
	Meters []struct {
		Power float64 `json:"power"`
		Total float64 `json:"total"`
	} `json:"meters"`
	Mac    string `json:"mac"`
	Relays []struct {
		IsOn bool `json:"ison"`
	} `json:"relays"`
	Temperature float64 `json:"temperature"`
	RamTotal    int64   `json:"ram_total"`
	RamFree     int64   `json:"ram_free"`
	FSSize      int64   `json:"fs_size"`
	FSFree      int64   `json:"fs_free"`
	Uptime      int64   `json:"uptime"`
}

func NewShelly(conf *collector.Config) collector.Collector {
	return &Shelly{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
		client: &http.Client{
			Timeout: conf.CollectInterval,
		},
		shellyUrl: os.Getenv("SHELLY_URL"),
	}
}

func (c *Shelly) Config() *collector.Config {
	return c.conf
}

func (c *Shelly) IsDaemon() bool { return false }

func (c *Shelly) Name() string {
	return CollectorShellyName
}

func parseMac(m string) string {
	m = strings.ToLower(m)
	m = m[0:2] + "-" + m[2:4] + "-" + m[4:6] + "-" + m[6:8] + "-" + m[8:10] + "-" + m[10:12]
	return m
}

func (c *Shelly) Collect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.shellyUrl+"/status", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	s := &Status{}
	err = json.Unmarshal(b, s)
	if err != nil {
		return err
	}
	if s.WifiSTA.IP == "" {
		return errors.New("invalid empty ip address")
	}
	if s.WifiSTA.SSID == "" {
		return errors.New("invalid empty ssid")
	}
	if len(s.Mac) != 12 {
		return fmt.Errorf("invalid mac address: %q", s.Mac)
	}
	tags := append(c.conf.Tagger.GetUnstable(c.conf.Host),
		"ip:"+s.WifiSTA.IP,
		"mac:"+parseMac(s.Mac),
		"shelly-model:plug",
	)
	now := time.Now()
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "temperature.celsius",
		Value:     s.Temperature,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      append(tags, "sensor:shelly"),
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:      "network.wireless.rssi.dbm",
		Value:     float64(s.WifiSTA.RSSI),
		Timestamp: now,
		Host:      c.conf.Host,
		Tags:      append(tags, "sensor:shelly", "ssid:"+s.WifiSTA.SSID),
	}, time.Minute)
	for i, meter := range s.Meters {
		meterTag := "meter:" + strconv.Itoa(i)
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:      "power.current",
			Value:     meter.Power,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      append(tags, meterTag),
		}, time.Minute)
		_ = c.measures.Count(&metrics.Sample{
			Name:      "power.total",
			Value:     meter.Power,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      append(tags, meterTag),
		})
	}
	for i, relay := range s.Relays {
		boolAsFloat := 0.
		if relay.IsOn {
			boolAsFloat = 1.
		}
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:      "power.on",
			Value:     boolAsFloat,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      append(tags, "relay:"+strconv.Itoa(i)),
		}, time.Minute)
	}
	return nil
}
