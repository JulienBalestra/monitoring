package catalog

import (
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq"
	"github.com/JulienBalestra/metrics/pkg/collector/load"
	"github.com/JulienBalestra/metrics/pkg/collector/memory"
	"github.com/JulienBalestra/metrics/pkg/collector/network"
	"github.com/JulienBalestra/metrics/pkg/collector/temperature"
)

func GetCollectorCatalog() map[string]func(*collector.Config) collector.Collector {
	return map[string]func(*collector.Config) collector.Collector{
		dnsmasq.CollectorDnsMasqName:         dnsmasq.NewDnsMasqReporter,
		load.CollectorLoadName:               load.NewLoadReporter,
		memory.CollectorMemoryName:           memory.NewMemoryReporter,
		network.CollectorARPName:             network.NewARPReporter,
		network.CollectorConntrackName:       network.NewConntrackReporter,
		network.CollectorStatisticsName:      network.NewStatisticsReporter,
		temperature.CollectorTemperatureName: temperature.NewTemperatureReporter,
	}
}
