package catalog

import (
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/bluetooth"
	"github.com/JulienBalestra/monitoring/pkg/collector/datadog"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dhcp"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dnslogs"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dnsqueries"
	"github.com/JulienBalestra/monitoring/pkg/collector/freebox"
	"github.com/JulienBalestra/monitoring/pkg/collector/golang"
	"github.com/JulienBalestra/monitoring/pkg/collector/google_home"
	"github.com/JulienBalestra/monitoring/pkg/collector/http_collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/load"
	"github.com/JulienBalestra/monitoring/pkg/collector/lunar"
	"github.com/JulienBalestra/monitoring/pkg/collector/memory"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/arp"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/conntrack"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/statistics"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/wireless"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/coredns"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/exporter"
	"github.com/JulienBalestra/monitoring/pkg/collector/prometheus/wireguard-stun/pubsub"
	"github.com/JulienBalestra/monitoring/pkg/collector/shelly"
	"github.com/JulienBalestra/monitoring/pkg/collector/tagger"
	"github.com/JulienBalestra/monitoring/pkg/collector/temperature/ddwrt"
	"github.com/JulienBalestra/monitoring/pkg/collector/temperature/raspberrypi"
	"github.com/JulienBalestra/monitoring/pkg/collector/uptime"
	"github.com/JulienBalestra/monitoring/pkg/collector/wireguard"
	"github.com/JulienBalestra/monitoring/pkg/collector/wl"
	datadogClient "github.com/JulienBalestra/monitoring/pkg/datadog"
	tagStore "github.com/JulienBalestra/monitoring/pkg/tagger"
	"gopkg.in/yaml.v2"
)

func CollectorCatalog() map[string]func(*collector.Config) collector.Collector {
	return map[string]func(*collector.Config) collector.Collector{
		dnsqueries.CollectorName:     dnsqueries.NewDNSMasqQueries,
		dnslogs.CollectorName:        dnslogs.NewDnsMasqLog,
		load.CollectorName:           load.NewLoad,
		memory.CollectorName:         memory.NewMemory,
		arp.CollectorName:            arp.NewARP,
		conntrack.CollectorName:      conntrack.NewConntrack,
		statistics.CollectorName:     statistics.NewStatistics,
		wireless.CollectorName:       wireless.NewWireless,
		shelly.CollectorName:         shelly.NewShelly,
		ddwrt.CollectorName:          ddwrt.NewTemperature,
		tagger.CollectorName:         tagger.NewTagger,
		wl.CollectorName:             wl.NewWL,
		datadog.CollectorName:        datadog.NewClient,
		dhcp.CollectorName:           dhcp.NewDNSMasqDHCP,
		raspberrypi.CollectorName:    raspberrypi.NewTemperature,
		wireguard.CollectorName:      wireguard.NewWireguard,
		uptime.CollectorName:         uptime.NewUptime,
		golang.CollectorName:         golang.NewGolang,
		exporter.CollectorName:       exporter.NewPrometheusExporter,
		coredns.CollectorName:        coredns.NewCoredns,
		google_home.CollectorName:    google_home.NewGoogleHome,
		http_collector.CollectorName: http_collector.NewHTTP,
		freebox.CollectorName:        freebox.NewFreebox,

		// WIP collectors:
		bluetooth.CollectorName: bluetooth.NewBluetooth,
		lunar.CollectorName:     lunar.NewAcaia,
		pubsub.CollectorName:    pubsub.NewPubSub,
	}
}

type ConfigFile struct {
	Collectors []Collector `yaml:"collectors"`
}

type Collector struct {
	Name     string            `yaml:"name"`
	Interval time.Duration     `yaml:"interval,omitempty"`
	Options  map[string]string `yaml:"options,omitempty"`
	Tags     []string          `yaml:"tags,omitempty"`
}

func ParseConfigFile(f string) (*ConfigFile, error) {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}
	c := &ConfigFile{}
	err = yaml.UnmarshalStrict(b, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func GenerateCollectorConfigFile(f string) error {
	fd, err := os.OpenFile(f, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()

	c := &ConfigFile{}
	catalog := CollectorCatalog()
	tag := tagStore.NewTagger()
	metricsClient := datadogClient.NewClient(&datadogClient.Config{})
	for name, newCollector := range catalog {
		coll := newCollector(&collector.Config{
			Tagger:        tag,
			MetricsClient: metricsClient,
		})
		c.Collectors = append(c.Collectors, Collector{
			Name:     name,
			Interval: coll.DefaultCollectInterval(),
			Options:  coll.DefaultOptions(),
			Tags:     coll.DefaultTags(),
		})
	}
	sort.Slice(c.Collectors, func(i, j int) bool {
		return c.Collectors[i].Name < c.Collectors[j].Name
	})
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	_, err = fd.Write(b)
	return err
}
