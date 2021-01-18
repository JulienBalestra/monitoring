package google_home

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/macvendor"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorName = "google-home"

	OptionIP = "ip"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	client *http.Client
	url    string
}

type EurekaInfo struct {
	Connected         bool    `json:"connected"`
	BuildVersion      string  `json:"build_version"`
	CastBuildRevision string  `json:"cast_build_revision"`
	HasUpdate         bool    `json:"has_update"`
	MacAddress        string  `json:"mac_address"`
	Name              string  `json:"name"`
	SignalLevel       float64 `json:"signal_level"`
	Noise             float64 `json:"noise_level"`
	SSID              string  `json:"ssid"`
	Uptime            float64 `json:"uptime"`
	SetupState        float64 `json:"setup_state"`
	BSSID             string  `json:"bssid"`
}

func NewGoogleHome(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		url: fmt.Sprintf("https://%s:8443/setup/eureka_info", conf.Options[OptionIP]),
	}
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) Collect(ctx context.Context) error {
	ipAddress, ok := c.conf.Options[OptionIP]
	if !ok {
		zap.L().Error("missing option", zap.String("options", OptionIP))
		return errors.New("missing option " + OptionIP)
	}
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request for URL %s returned HTTP status %s", req.URL.String(), resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	e := &EurekaInfo{}
	err = json.Unmarshal(b, e)
	if err != nil {
		return err
	}
	macAddress := macvendor.NormaliseMacAddress(e.MacAddress)
	bssid := macvendor.NormaliseMacAddress(e.BSSID)
	c.conf.Tagger.Update(macAddress,
		tagger.NewTagUnsafe("ip", ipAddress),
		tagger.NewTagUnsafe("ssid", e.SSID),
		tagger.NewTagUnsafe("bssid", bssid),
		tagger.NewTagUnsafe("vendor", macvendor.GetVendorWithMacOrUnknown(macAddress)),
	)
	now, tags := time.Now(), append(c.conf.Tagger.GetUnstable(macAddress), c.Tags()...)
	tags = append(tags,
		"mac:"+macAddress,
		"build-version:"+e.BuildVersion,
		"cast-build-revision:"+e.CastBuildRevision,
		"device-name:"+e.Name,
	)
	c.measures.Gauge(&metrics.Sample{
		Name:  "up.time",
		Value: e.Uptime,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	})

	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "network.wireless.rssi.dbm",
		Value: e.SignalLevel,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "network.wireless.noise",
		Value: e.Noise,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)

	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "google.home.has_update",
		Value: metrics.BoolToFloat(e.HasUpdate),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "google.home.connected",
		Value: metrics.BoolToFloat(e.Connected),
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	c.measures.GaugeDeviation(&metrics.Sample{
		Name:  "google.home.setup_state",
		Value: e.SetupState,
		Time:  now,
		Host:  c.conf.Host,
		Tags:  tags,
	}, c.conf.CollectInterval*c.conf.CollectInterval)
	return nil
}
