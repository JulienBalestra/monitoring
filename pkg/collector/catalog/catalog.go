package catalog

import (
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/ping"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/bluetooth"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/coredns"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/datadog"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/dnsmasq/dhcp"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/dnsmasq/dnslogs"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/dnsmasq/dnsqueries"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/etcd"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/freebox"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/golang"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/google_home"
	http_collector "github.com/JulienBalestra/monitoring/pkg/collector/collectors/http"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/load"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/lunar"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/memory"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/network/arp"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/network/conntrack"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/network/statistics"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/network/wireless"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/prometheus/exporter"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/shelly"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/tagger"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/temperature/ddwrt"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/temperature/raspberrypi"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/uptime"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/wireguard"
	etcdPeer "github.com/JulienBalestra/monitoring/pkg/collector/collectors/wireguard-stun/peer/etcd"
	etcdRegistry "github.com/JulienBalestra/monitoring/pkg/collector/collectors/wireguard-stun/registry/etcd"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/wl"
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
		etcdRegistry.CollectorName:   etcdRegistry.NewWireguardStunRegistryEtcd,
		etcdPeer.CollectorName:       etcdPeer.NewWireguardStunPeerEtcd,
		etcd.CollectorName:           etcd.NewEtcd,
		ping.CollectorName:           ping.NewPing,

		// WIP collectors:
		bluetooth.CollectorName: bluetooth.NewBluetooth,
		lunar.CollectorName:     lunar.NewAcaia,
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
