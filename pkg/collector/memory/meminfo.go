package memory

import (
	"context"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/metrics"
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
}

func NewMemory(conf *collector.Config) collector.Collector {
	return &Memory{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),
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

func (c *Memory) Config() *collector.Config {
	return c.conf
}

func (c *Memory) IsDaemon() bool { return false }

func (c *Memory) Name() string {
	return CollectorMemoryName
}

func (c *Memory) Collect(_ context.Context) error {
	b, err := ioutil.ReadFile(memInfoPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(b[:len(b)-1]), "\n")
	if len(lines) == 0 {
		return nil
	}
	now := time.Now()
	hostTags := c.conf.Tagger.Get(c.conf.Host)

	for i, line := range lines[3:] {
		raw := strings.Fields(line)
		if len(raw) != 3 {
			log.Printf("failed to parse meminfo line %d len(%d): %q : %q", i, len(raw), line, strings.Join(raw, ","))
			continue
		}
		metricCandidate := raw[0][:len(raw[0])-1]
		metricName := c.mapping[metricCandidate] // remove the trailing ":"
		if metricName == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw[1], 10)
		if err != nil {
			log.Printf("ignoring insupported meminfo metric value %s: %q: %v", metricName, raw[1], err)
			continue
		}

		c.measures.Gauge(&metrics.Sample{
			Name:      metricName,
			Value:     value * 1000, // reported in kB
			Timestamp: now,
			Host:      c.conf.Host,
			Tags:      hostTags,
		})
	}
	return nil
}
