package catalog

import (
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector/temperature/ddwrt"
	"github.com/JulienBalestra/monitoring/pkg/collector/temperature/raspberrypi"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/bluetooth"
	"github.com/JulienBalestra/monitoring/pkg/collector/datadog"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dhcp"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dnslogs"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/dnsqueries"
	"github.com/JulienBalestra/monitoring/pkg/collector/load"
	"github.com/JulienBalestra/monitoring/pkg/collector/lunar"
	"github.com/JulienBalestra/monitoring/pkg/collector/memory"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/arp"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/conntrack"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/statistics"
	"github.com/JulienBalestra/monitoring/pkg/collector/network/wireless"
	"github.com/JulienBalestra/monitoring/pkg/collector/shelly"
	"github.com/JulienBalestra/monitoring/pkg/collector/tagger"
	"github.com/JulienBalestra/monitoring/pkg/collector/wl"
	datadogClient "github.com/JulienBalestra/monitoring/pkg/datadog"
	tagStore "github.com/JulienBalestra/monitoring/pkg/tagger"
	"gopkg.in/yaml.v2"
)

func CollectorCatalog() map[string]func(*collector.Config) collector.Collector {
	return map[string]func(*collector.Config) collector.Collector{
		bluetooth.CollectorLoadName:          bluetooth.NewBluetooth,
		lunar.CollectorLoadName:              lunar.NewAcaia,
		dnsqueries.CollectorDnsMasqName:      dnsqueries.NewDNSMasqQueries,
		dnslogs.CollectorDnsMasqLogName:      dnslogs.NewDnsMasqLog,
		load.CollectorLoadName:               load.NewLoad,
		memory.CollectorMemoryName:           memory.NewMemory,
		arp.CollectorARPName:                 arp.NewARP,
		conntrack.CollectorConntrackName:     conntrack.NewConntrack,
		statistics.CollectorStatisticsName:   statistics.NewStatistics,
		wireless.CollectorWirelessName:       wireless.NewWireless,
		shelly.CollectorShellyName:           shelly.NewShelly,
		ddwrt.CollectorTemperatureName:       ddwrt.NewTemperature,
		tagger.CollectorName:                 tagger.NewTagger,
		wl.CollectorWLName:                   wl.NewWL,
		datadog.CollectorName:                datadog.NewClient,
		dhcp.CollectorDnsMasqName:            dhcp.NewDNSMasqDHCP,
		raspberrypi.CollectorTemperatureName: raspberrypi.NewTemperature,
	}
}

type ConfigFile struct {
	Collectors []Collector `yaml:"collectors"`
}

type Collector struct {
	Name     string            `yaml:"name"`
	Interval time.Duration     `yaml:"interval"`
	Options  map[string]string `yaml:"options"`
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
