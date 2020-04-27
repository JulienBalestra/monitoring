package load

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/datadog"
)

const (
	CollectorLoadName = "load"

	loadPath = "/proc/loadavg"
)

type Load struct {
	conf *collector.Config
}

func NewLoad(conf *collector.Config) collector.Collector {
	return &Load{
		conf: conf,
	}
}

func (c *Load) Config() *collector.Config {
	return c.conf
}

func (c *Load) IsDaemon() bool { return false }

func (c *Load) Name() string {
	return CollectorLoadName
}

func (c *Load) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	// example content:
	// 0.65 0.86 0.99 1/737 37114
	load, err := ioutil.ReadFile(loadPath)
	if err != nil {
		log.Printf("failed to parse metrics %s: %v", loadPath, err)
		return counters, gauges, err
	}
	// TODO use scan instead ?
	parts := strings.Fields(string(load))
	if len(parts) != 5 {
		return counters, gauges, fmt.Errorf("failed to parse %s: %q", parts, string(load))
	}
	load1, err := strconv.ParseFloat(parts[0], 10)
	if err != nil {
		return counters, gauges, err
	}
	load5, err := strconv.ParseFloat(parts[1], 10)
	if err != nil {
		return counters, gauges, err
	}
	load10, err := strconv.ParseFloat(parts[2], 10)
	if err != nil {
		return counters, gauges, err
	}

	// TODO do we need these following gauges ?
	//subPart := strings.Split(parts[3], "/")
	//if len(subPart) != 2 {
	//	return counters, gauges, fmt.Errorf("failed to parse %s: %q", subPart, parts[3])
	//}
	//lastProcess, err := strconv.ParseFloat(parts[4], 10)
	//if err != nil {
	//	return counters, gauges, err
	//}
	//
	//kernelScheduling, err := strconv.ParseFloat(subPart[0], 10)
	//if err != nil {
	//	return counters, gauges, err
	//}
	//kernelEntities, err := strconv.ParseFloat(subPart[1], 10)
	//if err != nil {
	//	return counters, gauges, err
	//}

	// newMetric is a convenient way to DRY the following gauges
	now, tags := time.Now(), c.conf.Tagger.Get(c.conf.Host)
	newMetric := func(name string, value float64) *datadog.Metric {
		return &datadog.Metric{
			Name:      name,
			Value:     value,
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      tags,
		}
	}
	return counters, append(gauges,
		newMetric("load.1", load1),
		newMetric("load.5", load5),
		newMetric("load.10", load10),
		//newMetric("process.last_id", lastProcess),
		//newMetric("kernel.scheduling", kernelScheduling),
		//newMetric("kernel.entities", kernelEntities),
	), nil
}
