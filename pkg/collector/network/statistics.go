package network

import (
	"context"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
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
	conf     *collector.Config
	measures *metrics.Measures

	statisticsFiles         map[string]*statisticFile
	statisticsFilesToUpdate time.Time
}

func NewStatistics(conf *collector.Config) collector.Collector {
	return &Statistics{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
	}
}

func (c *Statistics) Config() *collector.Config {
	return c.conf
}

func (c *Statistics) IsDaemon() bool { return false }

func (c *Statistics) Name() string {
	return CollectorStatisticsName
}

func (c *Statistics) Collect(_ context.Context) error {
	statistics, err := c.getStatisticsFiles()
	if err != nil {
		return err
	}

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
		_ = c.measures.Count(&metrics.Sample{
			Name:      "network.statistics." + statistic.fileName,
			Value:     i,
			Host:      c.conf.Host,
			Timestamp: now,
			Tags:      append(hostTags, "device:"+statistic.deviceName),
		})
	}
	return nil
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
