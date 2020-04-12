package load

import (
	"context"
	"fmt"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	loadPath = "/proc/loadavg"
)

type Load struct {
	conf *collecter.Config
}

func NewLoadReporter(conf *collecter.Config) *Load {
	return &Load{
		conf: conf,
	}
}

func (c *Load) collectMetrics() (datadog.GaugeList, error) {
	var gaugeLists datadog.GaugeList

	// example content:
	// 0.65 0.86 0.99 1/737 37114
	load, err := ioutil.ReadFile(loadPath)
	if err != nil {
		log.Printf("failed to parse metrics %s: %v", loadPath, err)
		return gaugeLists, err
	}
	// TODO use scan instead ?
	loadString := string(load[:len(load)-1])
	parts := strings.Split(loadString, " ")
	if len(parts) != 5 {
		return gaugeLists, fmt.Errorf("failed to parse %s: %q", parts, loadString)
	}
	subPart := strings.Split(parts[3], "/")
	if len(subPart) != 2 {
		return gaugeLists, fmt.Errorf("failed to parse %s: %q", subPart, parts[3])
	}
	load1, err := strconv.ParseFloat(parts[0], 10)
	if err != nil {
		return gaugeLists, err
	}
	load5, err := strconv.ParseFloat(parts[1], 10)
	if err != nil {
		return gaugeLists, err
	}
	load10, err := strconv.ParseFloat(parts[2], 10)
	if err != nil {
		return gaugeLists, err
	}
	lastProcess, err := strconv.ParseFloat(parts[4], 10)
	if err != nil {
		return gaugeLists, err
	}

	kernelScheduling, err := strconv.ParseFloat(subPart[0], 10)
	if err != nil {
		return gaugeLists, err
	}
	kernelEntities, err := strconv.ParseFloat(subPart[1], 10)
	if err != nil {
		return gaugeLists, err
	}

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
	return append(gaugeLists,
		newMetric("load.1", load1),
		newMetric("load.5", load5),
		newMetric("load.10", load10),
		newMetric("process.last_id", lastProcess),
		newMetric("kernel.scheduling", kernelScheduling),
		newMetric("kernel.entities", kernelEntities),
	), nil
}

func (c *Load) Collect(ctx context.Context) {
	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting load metrics every %s", c.conf.CollectInterval.String())
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of load collection")
			return

		case <-ticker.C:
			gauges, err := c.collectMetrics()
			if err != nil {
				log.Printf("failed load collection: %v", err)
				continue
			}
			gauges.Gauge(c.conf.MetricsCh)
			log.Printf("successfully run load collection")
		}
	}
}
