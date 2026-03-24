# Metrics System

## Metric Types

The system supports two Datadog metric types:

- **`gauge`** - A point-in-time value (e.g., current memory usage, load average)
- **`count`** - A delta between two consecutive samples over an interval (e.g., bytes transferred, DNS queries)

## Core Types

### Sample

A raw measurement from a collector:

```go
type Sample struct {
    Name  string
    Value float64
    Time  time.Time
    Host  string
    Tags  []string
}
```

### Series

The Datadog API format, produced from samples:

```go
type Series struct {
    Metric   string      `json:"metric"`
    Points   [][]float64 `json:"points"`     // [[timestamp, value], ...]
    Type     string      `json:"type"`       // "gauge" or "count"
    Interval float64     `json:"interval"`   // seconds (count only)
    Host     string      `json:"host"`
    Tags     []string    `json:"tags"`
}
```

## Measures

Each collector creates a `Measures` instance to submit metrics. Measures handles deduplication, delta computation, and series submission to the channel.

### Methods

#### `Gauge(sample)`

Sends a gauge series immediately. Every call produces a series on the channel.

#### `GaugeDeviation(sample, maxAge)`

Sends a gauge only if:
- The value changed since the last submission for this sample hash, OR
- More than `maxAge` has elapsed since the last submission

This is the most commonly used method. It reduces series volume by skipping unchanged values.

#### `Count(sample)`

Computes the delta between the current and previous sample (by hash). Only sends if the delta is positive. Returns an error on negative deltas (counter should not go backwards).

First call stores the sample without sending (needs two data points to compute a delta).

#### `CountWithNegativeReset(sample)`

Like `Count`, but silently resets on negative deltas instead of returning an error. Useful for counters that can reset (e.g., WireGuard transfer bytes after interface restart).

#### `Incr(sample)`

Accumulates the value on top of the previous sample, then computes a delta. Used for counters that report incremental values per collection (e.g., DNS query counts parsed from log lines).

#### `Purge()`

Removes stale entries from counter and deviation maps older than `maxAge` (default 12 hours). Should be called periodically by long-running collectors to prevent unbounded memory growth.

## Sample Hashing

Samples are identified by an FNV hash of:
- Metric name
- Host
- Sorted tags

This hash is used as the key for both counter tracking (previous values for delta computation) and deviation detection (previous values for change detection).

## Aggregation Store

The `AggregationStore` lives in the Datadog client goroutine and batches series before sending.

### Flow

1. Series arrive on `ChanSeries` from collectors
2. `Aggregate()` merges series with matching FNV hash (metric + host + type + interval + tags) by appending their points
3. On send-interval tick: all aggregated series are flushed via `SendSeries()`
4. On success: store is reset (pre-allocated to 90% of previous size)
5. On failure: garbage collection removes points older than 1 hour

### Garbage Collection

When a send fails, `GarbageCollect()` removes individual data points with timestamps older than 1 hour. Series with no remaining points are deleted entirely. This prevents unbounded memory growth during outages.

## Wire Protocol

Series are sent to `https://api.datadoghq.com/api/v1/series` as:

- **Method**: POST
- **Body**: JSON `{"series": [...]}` compressed with zlib (best compression)
- **Headers**: `Content-Type: application/json`, `Content-Encoding: deflate`
- **Authentication**: API key in query parameter

## Self-Instrumentation

### Per-Collector Meta-Metrics (every 5 minutes)

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `collector.series` | count | `collector:<name>` | Total series submitted |
| `collector.collections` | count | `collector:<name>`, `success:true/false` | Collection attempts |

### Datadog Client Lifecycle

| Metric | Type | Description |
|--------|------|-------------|
| `client.up` | gauge | Sent once on startup (value: 1) |
| `client.shutdown` | gauge | Sent on graceful shutdown (value: 1) |

### Datadog Client Stats (via datadog-client collector)

| Metric | Type | Description |
|--------|------|-------------|
| `client.sent.metrics.bytes` | count | Compressed bytes sent to Datadog |
| `client.sent.metrics.series` | count | Number of series sent |
| `client.metrics.errors` | count | Send failures |
| `client.metrics.store.aggregations` | count | Series merged in aggregation store |
| `client.sent.logs.bytes` | count | Log bytes sent to Datadog |
| `client.logs.errors` | count | Log send failures |
