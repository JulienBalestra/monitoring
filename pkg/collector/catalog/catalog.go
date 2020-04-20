package catalog

import (
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq"
	"github.com/JulienBalestra/metrics/pkg/collector/load"
	"github.com/JulienBalestra/metrics/pkg/collector/memory"
	"github.com/JulienBalestra/metrics/pkg/collector/network"
	"github.com/JulienBalestra/metrics/pkg/collector/tagger"
	"github.com/JulienBalestra/metrics/pkg/collector/temperature"
)

func CollectorCatalog() map[string]func(*collector.Config) collector.Collector {
	return map[string]func(*collector.Config) collector.Collector{
		dnsmasq.CollectorDnsMasqName:         dnsmasq.NewDnsMasq,
		dnsmasq.CollectorDnsMasqLogName:      dnsmasq.NewDnsMasqLog,
		load.CollectorLoadName:               load.NewLoad,
		memory.CollectorMemoryName:           memory.NewMemory,
		network.CollectorARPName:             network.NewARP,
		network.CollectorConntrackName:       network.NewConntrack,
		network.CollectorStatisticsName:      network.NewStatistics,
		network.CollectorWirelessName:        network.NewWireless,
		temperature.CollectorTemperatureName: temperature.NewTemperature,
		tagger.CollectorName:                 tagger.NewTagger,
	}
}
