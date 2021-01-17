package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorName = "shelly"

	optionEndpoint = "endpoint"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures
	client   *http.Client
}

type Status struct {
	WifiSTA struct {
		SSID string  `json:"ssid"`
		IP   string  `json:"ip"`
		RSSI float64 `json:"rssi"`
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
	RamTotal    float64 `json:"ram_total"`
	RamFree     float64 `json:"ram_free"`
	FSSize      float64 `json:"fs_size"`
	FSFree      float64 `json:"fs_free"`
	Uptime      float64 `json:"uptime"`
}

func NewShelly(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		client: &http.Client{
			Timeout: conf.CollectInterval,
		},
	}
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
		optionEndpoint: "http://192.168.1.2",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 5
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func parseMac(m string) string {
	m = strings.ToLower(m)
	m = m[0:2] + "-" + m[2:4] + "-" + m[4:6] + "-" + m[6:8] + "-" + m[8:10] + "-" + m[10:12]
	return m
}

func (c *Collector) Collect(ctx context.Context) error {
	shellyEndpoint, ok := c.conf.Options[optionEndpoint]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionEndpoint))
		return errors.New("missing option " + optionEndpoint)
	}
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shellyEndpoint+"/status", nil)
	cancel()
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
		"collector:"+CollectorName,
	)
	now := time.Now()
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "temperature.celsius",
		Value: s.Temperature,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  append(tags, "sensor:shelly"),
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "network.wireless.rssi.dbm",
		Value: s.WifiSTA.RSSI,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  append(tags, "ssid:"+s.WifiSTA.SSID),
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.free",
		Value: s.RamFree,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "memory.ram.total",
		Value: s.RamTotal,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "filesystem.free",
		Value: s.FSFree,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "filesystem.size",
		Value: s.FSSize,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, time.Minute)
	c.measures.Gauge(&metrics.Sample{
		Name:  "up.time",
		Value: s.Uptime,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	})

	for i, meter := range s.Meters {
		meterTag := "meter:" + strconv.Itoa(i)
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:  "power.current",
			Value: meter.Power,
			Time:  now,
			Host:  c.conf.Host,
			Tags:  append(tags, meterTag),
		}, time.Minute)
		_ = c.measures.Count(&metrics.Sample{
			Name:  "power.total",
			Value: meter.Total,
			Time:  now,
			Host:  c.conf.Host,
			Tags:  append(tags, meterTag),
		})
	}
	for i, relay := range s.Relays {
		c.measures.GaugeDeviation(&metrics.Sample{
			Name:  "power.on",
			Value: metrics.BoolToFloat(relay.IsOn),
			Time:  now,
			Host:  c.conf.Host,
			Tags:  append(tags, "relay:"+strconv.Itoa(i)),
		}, time.Minute)
	}
	return nil
}
