package statistics

import (
	"context"
	"errors"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"go.uber.org/zap"
)

const (
	CollectorName = "network-statistics"

	optionSysClassPath = "sys-class-net-path"
)

type statisticFile struct {
	fileName   string
	deviceName string
}

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	statisticsFiles         map[string]*statisticFile
	statisticsFilesToUpdate time.Time
}

func NewStatistics(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
	}
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{
		optionSysClassPath: "/sys/class/net/",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 10
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) Collect(_ context.Context) error {
	statistics, err := c.getStatisticsFiles()
	if err != nil {
		return err
	}

	hostTags := c.Tags()
	now := time.Now()
	for metricPath, statistic := range statistics {
		// TODO use a buffer
		metric, err := ioutil.ReadFile(metricPath)
		if err != nil {
			zap.L().Error("failed to read metrics from statistics",
				zap.String("metricPath", metricPath), zap.Error(err))
			continue
		}

		i, err := strconv.ParseFloat(string(metric[:len(metric)-1]), 10)
		if err != nil {
			zap.L().Error("failed to parse metrics from statistics",
				zap.String("metricPath", metricPath), zap.Error(err))
			continue
		}
		_ = c.measures.Count(&metrics.Sample{
			Name:  "network.statistics." + statistic.fileName,
			Value: i,
			Host:  c.conf.Host,
			Time:  now,
			Tags:  append(hostTags, "device:"+statistic.deviceName),
		})
	}
	return nil
}

func (c *Collector) getStatisticsFiles() (map[string]*statisticFile, error) {
	now := time.Now()
	if now.Before(c.statisticsFilesToUpdate) && c.statisticsFiles != nil {
		return c.statisticsFiles, nil
	}

	devicesPath, ok := c.conf.Options[optionSysClassPath]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionSysClassPath))
		return nil, errors.New("missing option " + optionSysClassPath)
	}

	devices, err := ioutil.ReadDir(devicesPath)
	if err != nil {
		return nil, err
	}

	statisticsFiles := make(map[string]*statisticFile)
	for _, device := range devices {
		deviceName := device.Name()
		statisticsPath := devicesPath + deviceName + "/statistics/"
		statistics, err := ioutil.ReadDir(statisticsPath)
		if err != nil {
			zap.L().Error("failed to read statistics for device",
				zap.String("device", statisticsPath), zap.Error(err))
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
