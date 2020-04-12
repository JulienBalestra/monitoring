package network

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"io/ioutil"
	"log"
	"strconv"
	"time"
)

const (
	devicesPath = "/sys/class/net/"
)

type statisticFile struct {
	fileName   string
	deviceName string
}

type Statistics struct {
	conf *collecter.Config

	statisticsFiles         map[string]*statisticFile
	statisticsFilesToUpdate time.Time
}

func NewStatisticsReporter(conf *collecter.Config) *Statistics {
	return &Statistics{
		conf: conf,
	}
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

func (c *Statistics) collectMetricsCount() (datadog.CounterMap, error) {
	statistics, err := c.getStatisticsFiles()
	if err != nil {
		return nil, err
	}

	metricsByPath := make(datadog.CounterMap)
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
		metricsByPath[metricPath] = m
	}
	return metricsByPath, nil
}

func (c *Statistics) Collect(ctx context.Context) {
	var counters datadog.CounterMap

	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	log.Printf("collecting network/statistics metrics every %s", c.conf.CollectInterval.String())
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of network/statistics collection")
			return

		case <-ticker.C:
			newCounters, err := c.collectMetricsCount()
			if err != nil {
				log.Printf("failed network/statistics collection: %v", err)
				continue
			}
			if counters != nil {
				counters.Count(c.conf.MetricsCh, newCounters)
			}
			counters = newCounters
			log.Printf("successfully run network/statistics collection")
		}
	}
}
