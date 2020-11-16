package catalog

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/bluetooth"
	"github.com/JulienBalestra/monitoring/pkg/collector/datadog"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq"
	"github.com/JulienBalestra/monitoring/pkg/collector/load"
	"github.com/JulienBalestra/monitoring/pkg/collector/lunar"
	"github.com/JulienBalestra/monitoring/pkg/collector/memory"
	"github.com/JulienBalestra/monitoring/pkg/collector/network"
	"github.com/JulienBalestra/monitoring/pkg/collector/shelly"
	"github.com/JulienBalestra/monitoring/pkg/collector/tagger"
	"github.com/JulienBalestra/monitoring/pkg/collector/temperature"
	"github.com/JulienBalestra/monitoring/pkg/collector/wl"
	"gopkg.in/yaml.v2"
)

func CollectorCatalog() map[string]func(*collector.Config) collector.Collector {
	return map[string]func(*collector.Config) collector.Collector{
		bluetooth.CollectorLoadName:          bluetooth.NewBluetooth,
		lunar.CollectorLoadName:              lunar.NewAcaia,
		dnsmasq.CollectorDnsMasqName:         dnsmasq.NewDnsMasq,
		dnsmasq.CollectorDnsMasqLogName:      dnsmasq.NewDnsMasqLog,
		load.CollectorLoadName:               load.NewLoad,
		memory.CollectorMemoryName:           memory.NewMemory,
		network.CollectorARPName:             network.NewARP,
		network.CollectorConntrackName:       network.NewConntrack,
		network.CollectorStatisticsName:      network.NewStatistics,
		network.CollectorWirelessName:        network.NewWireless,
		shelly.CollectorShellyName:           shelly.NewShelly,
		temperature.CollectorTemperatureName: temperature.NewTemperature,
		tagger.CollectorName:                 tagger.NewTagger,
		wl.CollectorWLName:                   wl.NewWL,
		datadog.CollectorName:                datadog.NewClient,
	}
}

type ConfigFile struct {
	Collectors map[string]Collector `yaml:"collectors"`
}

type Collector struct {
	Interval time.Duration `yaml:"interval"`
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
	catalog := CollectorCatalog()
	c := &ConfigFile{
		Collectors: make(map[string]Collector, len(catalog)),
	}
	for name := range catalog {
		c.Collectors[name] = Collector{}
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	_, err = fd.Write(b)
	return err
}
