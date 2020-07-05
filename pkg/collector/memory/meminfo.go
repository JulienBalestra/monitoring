package memory

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorMemoryName = "memory"

	memInfoPath        = "/proc/meminfo"
	memoryMetricPrefix = "memory."
)

/* cat /proc/meminfo
        total:    used:    free:  shared: buffers:  cached:
Mem:  259969024 55738368 204230656        0  4722688 21417984
Swap:        0        0        0
MemTotal:         253876 kB
MemFree:          199444 kB
MemShared:             0 kB
Buffers:            4612 kB
Cached:            20916 kB
SwapCached:            0 kB
Active:            18756 kB
Inactive:          15676 kB
MemAvailable:     216360 kB
Active(anon):       8904 kB
Inactive(anon):        0 kB
Active(file):       9852 kB
Inactive(file):    15676 kB
Unevictable:           0 kB
Mlocked:               0 kB
HighTotal:        131072 kB
HighFree:         108784 kB
LowTotal:         122804 kB
LowFree:           90660 kB
SwapTotal:             0 kB
SwapFree:              0 kB
Dirty:                 0 kB
Writeback:             0 kB
AnonPages:          8904 kB
Mapped:             7260 kB
Shmem:                 0 kB
Slab:               8120 kB
SReclaimable:       1636 kB
SUnreclaim:         6484 kB
KernelStack:         704 kB
PageTables:          336 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:      126936 kB
Committed_AS:      16324 kB
VmallocTotal:    1949696 kB
VmallocUsed:           0 kB
VmallocChunk:          0 kB
*/

type Memory struct {
	conf     *collector.Config
	mapping  map[string]string
	measures *metrics.Measures

	endline []byte
}

func newMemory(conf *collector.Config) *Memory {
	return &Memory{
		endline:  []byte(" kB\n"),
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		// TODO this is all the available metrics, some are commented for random reasons
		mapping: map[string]string{
			"MemTotal":  memoryMetricPrefix + "total",
			"MemFree":   memoryMetricPrefix + "free",
			"MemShared": memoryMetricPrefix + "shared",
			"Buffers":   memoryMetricPrefix + "buffer",
			"Cached":    memoryMetricPrefix + "cached",
			//"SwapCached":   memoryMetricPrefix + "swap.cached",
			"Active":       memoryMetricPrefix + "active",
			"Inactive":     memoryMetricPrefix + "inactive",
			"MemAvailable": memoryMetricPrefix + "available",
			//"Active(anon)":   memoryMetricPrefix + "anon.active",
			//"Inactive(anon)": memoryMetricPrefix + "anon.inactive",
			"Active(file)":   memoryMetricPrefix + "file.active",
			"Inactive(file)": memoryMetricPrefix + "file.inactive",
			//"Unevictable":    memoryMetricPrefix + "unevictable",
			//"Mlocked":        memoryMetricPrefix + "mlocked",
			//"HighTotal":      memoryMetricPrefix + "high.total",
			//"HighFree":       memoryMetricPrefix + "high.free",
			//"LowTotal":       memoryMetricPrefix + "low.total",
			//"LowFree":        memoryMetricPrefix + "low.free",
			//"SwapTotal": memoryMetricPrefix + "swap.total",
			//"SwapFree":  memoryMetricPrefix + "swap.free",
			"Dirty": memoryMetricPrefix + "dirty",
			//"Writeback":      memoryMetricPrefix + "writeback",
			//"AnonPages":      memoryMetricPrefix + "anon.pages",
			//"Mapped":         memoryMetricPrefix + "mapped",
			//"Shmem":          memoryMetricPrefix + "shmem",
			//"Slab":           memoryMetricPrefix + "slab",
			//"SReclaimable":   memoryMetricPrefix + "sreclaimable",
			//"SUnreclaim":     memoryMetricPrefix + "sunreclaim",
			//"KernelStack":    memoryMetricPrefix + "kernel_stack",
			//"PageTables":     memoryMetricPrefix + "page_tables",
			//"NFS_Unstable":   memoryMetricPrefix + "nfs_unstable",
			//"Bounce":         memoryMetricPrefix + "bounce",
			//"WritebackTmp":   memoryMetricPrefix + "writeback_tmp",
			//"CommitLimit":    memoryMetricPrefix + "commit_limit",
			//"Committed_AS":   memoryMetricPrefix + "committed_as",
			//"VmallocTotal":   memoryMetricPrefix + "vmalloc.total",
			//"VmallocUsed":    memoryMetricPrefix + "vmalloc.used",
			//"VmallocChunk":   memoryMetricPrefix + "vmalloc.chunck",
		},
	}
}

func NewMemory(conf *collector.Config) collector.Collector {
	return newMemory(conf)
}

func (c *Memory) Config() *collector.Config {
	return c.conf
}

func (c *Memory) IsDaemon() bool { return false }

func (c *Memory) Name() string {
	return CollectorMemoryName
}

func (c *Memory) collect(f string) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	now := time.Now()
	hostTags := c.conf.Tagger.Get(c.conf.Host)
	for {
		// TODO improve this reader
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(line) < 1 {
			continue
		}
		// dd-wrt starts its meminfo file like this:
		//        total:    used:    free:  shared: buffers:  cached:
		if line[0] == ' ' {
			continue
		}
		i := bytes.Index(line, c.endline)
		if i == -1 {
			continue
		}
		lineToParse := line[:i]
		fields := bytes.Fields(lineToParse)
		if len(fields) != 2 {
			continue
		}
		if len(fields[0]) < 2 {
			continue
		}
		metricCandidate := string(fields[0][:len(fields[0])-1])
		metricName := c.mapping[metricCandidate] // remove the trailing ":"
		if metricName == "" {
			continue
		}
		metricsValue := string(fields[1])
		value, err := strconv.ParseFloat(metricsValue, 10)
		if err != nil {
			zap.L().Error("ignoring insupported meminfo metric value",
				zap.String("metricName", metricName),
				zap.Error(err),
				zap.String("field", metricsValue),
			)
			continue
		}

		c.measures.GaugeDeviation(&metrics.Sample{
			Name:      metricName,
			Value:     value * 1000, // reported in kB
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		}, c.conf.CollectInterval*3)

	}
}

func (c *Memory) Collect(_ context.Context) error {
	return c.collect(memInfoPath)
}
