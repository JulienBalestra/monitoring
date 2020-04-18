package network

import (
	"context"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/datadog"
)

const (
	CollectorStatisticsName = "network-statistics"

	devicesPath = "/sys/class/net/"
)

type statisticFile struct {
	fileName   string
	deviceName string
}

type Statistics struct {
	conf *collector.Config

	statisticsFiles         map[string]*statisticFile
	statisticsFilesToUpdate time.Time
}

func NewStatisticsReporter(conf *collector.Config) collector.Collector {
	return &Statistics{
		conf: conf,
	}
}

func (c *Statistics) Config() *collector.Config {
	return c.conf
}

func (c *Statistics) Name() string {
	return CollectorStatisticsName
}

func (c *Statistics) Collect(_ context.Context) (datadog.Counter, datadog.Gauge, error) {
	var counters datadog.Counter
	var gauges datadog.Gauge

	statistics, err := c.getStatisticsFiles()
	if err != nil {
		return counters, gauges, err
	}

	counters = make(datadog.Counter)
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	now := time.Now()
	for metricPath, statistic := range statistics {
		metric, err := ioutil.ReadFile(metricPath)
		if err != nil {
			log.Printf("failed to read metrics from statistics %s: %v", metricPath, err)
			continue
		}

		i, err := strconv.ParseFloat(string(metric[:len(metric)-1]), 10)
		if err != nil {
			log.Printf("failed to parse metrics from statistics %s: %v", metricPath, err)
			continue
		}
		m := &datadog.Metric{
			Name:      "network.statistics." + statistic.fileName,
			Value:     i,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      append(hostTags, "device:"+statistic.deviceName),
		}
		counters[metricPath] = m
	}
	return counters, gauges, nil
}

func (c *Statistics) getStatisticsFiles() (map[string]*statisticFile, error) {
	now := time.Now()
	if now.Before(c.statisticsFilesToUpdate) && c.statisticsFiles != nil {
		return c.statisticsFiles, nil
	}

	devices, err := ioutil.ReadDir(devicesPath)
	if err != nil {
		return nil, err
	}

	statisticsFiles := make(map[string]*statisticFile)
	for _, device := range devices {
		// TODO use a use usable buffer of bytes or something else cool
		deviceName := device.Name()
		statisticsPath := devicesPath + deviceName + "/statistics/"
		statistics, err := ioutil.ReadDir(statisticsPath)
		if err != nil {
			log.Printf("failed to read statistics for device %s: %v", statisticsPath, err)
			continue
		}
		for _, statistic := range statistics {
			fileName := statistic.Name()
			statisticsFiles[statisticsPath+fileName] = &statisticFile{
				fileName:   fileName,
				deviceName: deviceName,
			}
		}
	}
	// cache this for the added duration
	c.statisticsFilesToUpdate = now.Add(time.Minute * 5)
	c.statisticsFiles = statisticsFiles
	return statisticsFiles, nil
}
