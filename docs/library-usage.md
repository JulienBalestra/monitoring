# Library Usage

The `datadog`, `metrics`, and `tagger` packages can be imported independently into your own Go programs. You don't need to run the full monitoring daemon to use them.

## Quick Start

Full example at `pkg/datadog/example/example.go`:

```go
package main

import (
    "context"
    "sync"
    "time"

    "github.com/JulienBalestra/monitoring/pkg/datadog"
    "github.com/JulienBalestra/monitoring/pkg/metrics"
)

func main() {
    ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*2)
    defer cancel()

    // 1. Create a client
    c := datadog.NewClient(&datadog.Config{
        Host:          "my-host",
        DatadogAPIKey: "your-api-key",
        DatadogAPPKey: "your-app-key",
        SendInterval:  time.Second * 60,
    })

    // 2. Build a series manually
    series := []metrics.Series{
        {
            Metric: "custom.metrics",
            Points: [][]float64{
                {float64(time.Now().Unix()), 42},
            },
            Type: metrics.TypeGauge,
            Host: "my-host",
            Tags: []string{"env:production"},
        },
    }

    // 3. Send synchronously
    _ = c.SendSeries(ctx, series)

    // 4. Or run the client in the background for aggregation
    wg := sync.WaitGroup{}
    defer wg.Wait()
    wg.Add(1)
    go func() {
        c.Run(ctx)
        wg.Done()
    }()

    // 5. Feed metrics via the channel - the client aggregates before sending
    ticker := time.NewTicker(time.Second * 30)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.ChanSeries <- series[0]
        }
    }
}
```

## Client API

### Config

```go
c := datadog.NewClient(&datadog.Config{
    Host:          "hostname",        // Required: appears on all metrics
    DatadogAPIKey: "api-key",         // Required: Datadog API key
    DatadogAPPKey: "app-key",         // Required for UpdateHostTags
    SendInterval:  time.Second * 60,  // Batch send interval (min 5s, default 60s)
    ChanSize:      0,                 // Buffer size for ChanSeries (0 = unbuffered)
    ClientMetrics: &datadog.ClientMetrics{}, // Optional: track send statistics
})
```

### Methods

| Method | Description |
|--------|-------------|
| `Run(ctx)` | Background loop: reads from `ChanSeries`, aggregates, sends every `SendInterval`. Flushes pending series on context cancellation. Run in a goroutine. |
| `SendSeries(ctx, []Series)` | Synchronous send. Compresses with zlib and POSTs to Datadog API. |
| `SendLogs(ctx, *bytes.Buffer)` | Send logs to Datadog Logs API. |
| `UpdateHostTags(ctx, []string)` | Update host tags in Datadog (requires APP key). |
| `MetricClientUp(host, tags...)` | Send a `client.up` gauge (value 1) via the channel. |
| `MetricClientShutdown(ctx, host, tags...)` | Send a `client.shutdown` gauge synchronously. |

### Client Statistics

When `ClientMetrics` is provided in the config, the client tracks:

| Field | Description |
|-------|-------------|
| `SentSeriesBytes` | Compressed bytes sent |
| `SentSeries` | Number of series sent |
| `SentSeriesErrors` | Send failures |
| `StoreAggregations` | Series merged during aggregation |
| `SentLogsBytes` | Log bytes sent |
| `SentLogsErrors` | Log send failures |

## Using Measures

`Measures` provides a higher-level API on top of the channel, handling deduplication and delta computation. See [Metrics System](metrics-system.md) for full details.

```go
m := metrics.NewMeasures(client.ChanSeries)

// Gauge - send immediately
m.Gauge(&metrics.Sample{
    Name:  "my.gauge",
    Value: 42,
    Time:  time.Now(),
    Host:  "my-host",
    Tags:  []string{"env:prod"},
})

// GaugeDeviation - only send if value changed or maxAge elapsed
m.GaugeDeviation(&metrics.Sample{
    Name:  "my.temperature",
    Value: 22.5,
    Time:  time.Now(),
    Host:  "my-host",
    Tags:  []string{"sensor:cpu"},
}, time.Minute*5)

// Count - compute delta between consecutive samples
m.Count(&metrics.Sample{
    Name:  "my.counter",
    Value: 1000, // absolute value; delta computed from previous call
    Time:  time.Now(),
    Host:  "my-host",
})
```

| Method | Use When |
|--------|----------|
| `Gauge` | Every reading matters |
| `GaugeDeviation` | Value may be stable; skip unchanged values |
| `Count` | Monotonically increasing counter (computes delta) |
| `CountWithNegativeReset` | Counter that can reset to zero |
| `Incr` | Incremental values that accumulate before delta |

## Using the Tagger

The tagger provides optional dynamic tag management:

```go
import "github.com/JulienBalestra/monitoring/pkg/tagger"

t := tagger.NewTagger()

// Store tags for an entity
tag, _ := tagger.NewTag("role", "web")
t.Update("my-host", tag)

// Retrieve tags
tags := t.GetUnstable("my-host") // ["role:web"]

// Use with metrics
m.Gauge(&metrics.Sample{
    Name:  "my.metric",
    Value: 1,
    Time:  time.Now(),
    Host:  "my-host",
    Tags:  append(t.GetUnstable("my-host"), "extra:tag"),
})
```

## Design Notes

- **Channel-based**: Metrics are submitted asynchronously via `ChanSeries`. Multiple goroutines can write safely.
- **Aggregation**: The `Run()` loop merges series with the same metric name, host, type, interval, and tags before sending. This reduces API calls.
- **Graceful shutdown**: Cancel the context passed to `Run()`. It flushes pending series with a 5-second timeout before returning.
- **Compression**: All payloads are zlib-compressed (best compression) before sending.
- **Send interval**: Must be >= 5 seconds. If set below this, defaults to 60 seconds.
