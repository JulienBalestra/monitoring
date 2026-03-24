# Adding a Collector

This guide walks through implementing a new collector for the monitoring system.

## Prerequisites

- Go development environment
- Familiarity with the `Collector` interface in `pkg/collector/interface.go`

## Step-by-Step

### 1. Create the Package

Create a new directory under `pkg/collector/collectors/`. Use a category subdirectory if appropriate:

```
pkg/collector/collectors/<category>/<name>/
```

### 2. Implement the Collector

Use the `memory` collector as a minimal template for periodic collectors, or `dnsmasq/dnslogs` for daemon collectors.

```go
package mymetric

import (
    "context"
    "time"

    "github.com/JulienBalestra/monitoring/pkg/collector"
    "github.com/JulienBalestra/monitoring/pkg/metrics"
)

const CollectorName = "my-metric"

type MyMetric struct {
    conf     *collector.Config
    measures *metrics.Measures
}

func NewMyMetric(conf *collector.Config) collector.Collector {
    return collector.WithDefaults(&MyMetric{
        conf:     conf,
        measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),
    })
}

func (c *MyMetric) DefaultOptions() map[string]string {
    return map[string]string{
        "some-option": "default-value",
    }
}

func (c *MyMetric) DefaultCollectInterval() time.Duration {
    return time.Second * 30
}

func (c *MyMetric) DefaultTags() []string {
    return []string{
        "collector:" + CollectorName,
    }
}

func (c *MyMetric) Config() *collector.Config {
    return c.conf
}

func (c *MyMetric) IsDaemon() bool {
    return false
}

func (c *MyMetric) Name() string {
    return CollectorName
}

func (c *MyMetric) Tags() []string {
    return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *MyMetric) SubmittedSeries() float64 {
    return c.measures.GetTotalSubmittedSeries()
}

func (c *MyMetric) Collect(ctx context.Context) error {
    now := time.Now()
    tags := c.Tags()

    // Collect your metric value
    value := 42.0

    c.measures.GaugeDeviation(&metrics.Sample{
        Name:  "my.metric.name",
        Value: value,
        Time:  now,
        Host:  c.conf.Host,
        Tags:  tags,
    }, c.conf.CollectInterval*3)

    return nil
}
```

### 3. Register in the Catalog

Add the import and entry in `pkg/collector/catalog/catalog.go`:

```go
import (
    mymetric "github.com/JulienBalestra/monitoring/pkg/collector/collectors/<category>/<name>"
)

func CollectorCatalog() map[string]func(*collector.Config) collector.Collector {
    return map[string]func(*collector.Config) collector.Collector{
        // ... existing collectors ...
        mymetric.CollectorName: mymetric.NewMyMetric,
    }
}
```

### 4. Write Tests

Add unit tests alongside your collector. See existing patterns:
- `pkg/collector/collectors/wl/wl_test.go` - parsing command output
- `pkg/conntrack/conntrack_test.go` - parsing file formats
- `pkg/tagger/tags_test.go` - tag operations

```bash
go test -v -race ./pkg/collector/collectors/<category>/<name>/...
```

### 5. Add to Config and Test

Add your collector to a config YAML to test end-to-end:

```yaml
collectors:
  - name: my-metric
    options:
      some-option: "custom-value"
```

## Choosing the Right Metric Method

| Method | Use When | Example |
|--------|----------|---------|
| `Gauge` | Value always changes and every reading matters | uptime seconds |
| `GaugeDeviation` | Value may be stable; skip unchanged values | memory usage, temperature |
| `Count` | Monotonically increasing counter | bytes transferred, request counts |
| `CountWithNegativeReset` | Counter that can reset to zero | WireGuard transfer bytes |
| `Incr` | Incremental values per collection (accumulate then delta) | DNS queries from log lines |

`GaugeDeviation` is the most commonly used method. Pass a `maxAge` (typically `collectInterval * 3`) to force-send even if unchanged.

## Periodic vs Daemon Collectors

### Periodic (`IsDaemon() == false`)

The framework calls `Collect()` on a ticker. Just implement a single collection pass:

```go
func (c *MyMetric) Collect(ctx context.Context) error {
    // Read data, submit metrics, return
    return nil
}
```

### Daemon (`IsDaemon() == true`)

`Collect()` runs for the lifetime of the context. You manage your own loop:

```go
func (c *MyMetric) Collect(ctx context.Context) error {
    ticker := time.NewTicker(c.conf.CollectInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            // collect and submit
        }
    }
}
```

Use daemon mode when you need to tail files, maintain persistent connections, or handle events.

## Using the Tagger

### Enriching Tags for Other Collectors

Store tags so other collectors can look them up:

```go
tag, _ := tagger.NewTag("lease", "my-device-name")
c.conf.Tagger.Update("192.168.1.100", tag)
```

### Consuming Tags from Other Collectors

Read tags stored by other collectors:

```go
tags := c.conf.Tagger.GetUnstable("192.168.1.100")
// Returns e.g. ["lease:my-device-name", "vendor:Apple"]
```

Use `GetUnstableWithDefault` to provide fallback tags when an entity is not found:

```go
defaultTag := tagger.NewTagUnsafe("lease", tagger.MissingTagValue)
tags := c.conf.Tagger.GetUnstableWithDefault("192.168.1.100", defaultTag)
```

### Cross-Collector Enrichment Pattern

This is a key architectural pattern:
1. `dnsmasq-dhcp` reads lease files and stores `lease:<hostname>` tags for MAC/IP entities
2. `network-arp` reads those tags when reporting ARP entries, adding lease context
3. `network-conntrack` reads both ARP and DHCP tags for connection tracking metrics

## Interface Checklist

- [ ] `Config()` - return stored config
- [ ] `Name()` - return `CollectorName` constant
- [ ] `IsDaemon()` - false for periodic, true for long-running
- [ ] `DefaultOptions()` - map of option keys to default values
- [ ] `DefaultCollectInterval()` - sensible default duration
- [ ] `DefaultTags()` - at minimum `[]string{"collector:" + CollectorName}`
- [ ] `Tags()` - standard: `append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)`
- [ ] `SubmittedSeries()` - return `c.measures.GetTotalSubmittedSeries()`
- [ ] `Collect(ctx)` - the core collection logic
