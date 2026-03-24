# Architecture

## Overview

The monitoring application is a single-binary daemon that runs multiple metric collectors concurrently and sends their output to the Datadog API. Each collector runs in its own goroutine and submits metrics through a shared channel to a central Datadog client.

```
                          +-------------------+
                          |   config.yaml     |
                          +--------+----------+
                                   |
                          +--------v----------+
                          |  monitoring.Start  |
                          +--------+----------+
                                   |
              +--------------------+--------------------+
              |                    |                     |
     +--------v------+   +--------v------+   +----------v----+
     | Collector A   |   | Collector B   |   | Collector N   |
     | (goroutine)   |   | (goroutine)   |   | (goroutine)   |
     +--------+------+   +--------+------+   +----------+----+
              |                    |                     |
              |   measures.Gauge / Count / etc.         |
              |                    |                     |
              +--------------------+--------------------+
                                   |
                          +--------v----------+
                          |    ChanSeries     |
                          |  (buffered chan)   |
                          +--------+----------+
                                   |
                          +--------v----------+
                          | AggregationStore  |
                          | (batch + dedup)   |
                          +--------+----------+
                                   |
                          +--------v----------+
                          | Datadog Client    |
                          | (zlib POST)       |
                          +--------+----------+
                                   |
                          +--------v----------+
                          | Datadog API       |
                          | /api/v1/series    |
                          +-------------------+
```

## Startup Sequence

1. **`main/main.go`** creates a context and the root Cobra command.
2. **`cmd/root.go`** parses CLI flags, resolves env vars for API keys, and creates a `monitoring.Config`.
3. **`monitoring.NewMonitoring()`**:
   - Parses the YAML config file via `catalog.ParseConfigFile()`.
   - Creates the Datadog client.
   - Configures zap logging with an optional Datadog log forwarder sink (`datadog://zap`).
4. **`monitoring.Start()`**:
   - Launches the Datadog client goroutine (`client.Run()`).
   - Iterates the collector catalog, matching entries from the config file.
   - Starts each configured collector in its own goroutine via `collector.RunCollection()`.
   - Sends a `client.up` metric.
   - Waits for context cancellation or a collector error.
   - On shutdown: sends `client.shutdown`, waits for collectors to finish, then stops the Datadog client (flushing pending series).

## Collector Lifecycle

### Periodic Collectors (`IsDaemon() == false`)

`RunCollection()` creates a ticker at `CollectInterval` and calls `Collect()` on each tick. Every 5 minutes it also emits meta-metrics:

- `collector.series` (count) - total series submitted by this collector
- `collector.collections` (count, tagged `success:true/false`) - collection attempts

### Daemon Collectors (`IsDaemon() == true`)

`RunCollection()` calls `Collect()` once. The collector manages its own event loop internally (e.g., tailing a log file or polling with a custom interval). It must respect `ctx.Done()` for shutdown.

### Configuration Merging

`WithDefaults()` merges user-provided config with collector defaults:
- **Options**: user options take priority; missing keys filled from `DefaultOptions()`
- **Tags**: user tags merged with `DefaultTags()` (no duplicates)
- **Interval**: user interval used if non-zero, otherwise `DefaultCollectInterval()`

## Signal Handling

The application registers OS signal handlers via `signals.NotifySignals()`. On receiving an OS signal, it prints the tagger state (all entities and their tags) via `m.Tagger.Print()`, then cancels the context to trigger graceful shutdown.

## Tagger System

The tagger (`pkg/tagger/`) provides dynamic tag enrichment across collectors. It stores tags in a three-level hierarchy:

```
entity -> tagKey -> tagValue -> "key:value"
```

Operations:
- **Add** - Append values for a key (supports multiple values per key per entity)
- **Update** - Replace all values for a key
- **Replace** - Replace all tags for an entity
- **Get / GetUnstable** - Retrieve tags (sorted / unsorted)
- **GetUnstableWithDefault** - Retrieve tags with fallback defaults for missing keys

Cross-collector enrichment example:
- `dnsmasq-dhcp` populates lease names for MAC/IP entities
- `network-arp` reads those tags to enrich ARP metrics with lease information
- `network-conntrack` reads both ARP and DHCP tags for connection tracking metrics

## Package Map

| Package | Responsibility |
|---------|---------------|
| `main/` | Entry point |
| `cmd/` | CLI flags and root Cobra command |
| `pkg/monitoring/` | Orchestrator: starts client and collectors, handles shutdown |
| `pkg/collector/` | Collector interface, `RunCollection()`, `WithDefaults()` |
| `pkg/collector/catalog/` | Factory map of all collectors, YAML config parsing |
| `pkg/collector/collectors/*/` | Individual collector implementations |
| `pkg/metrics/` | Sample, Series, Measures (gauge/count methods), AggregationStore |
| `pkg/datadog/` | HTTP client for Datadog API (series, logs, host tags) |
| `pkg/datadog/forward/` | Zap log sink that forwards logs to Datadog |
| `pkg/tagger/` | Dynamic tag store with entity/key/value hierarchy |
| `pkg/conntrack/` | Linux `/proc/net/ip_conntrack` parser |
| `pkg/macvendor/` | MAC address vendor lookup (generated database) |
