# Monitoring Documentation

A lightweight Go monitoring daemon with a plugin-based collector architecture. It periodically collects system, network, and IoT metrics from various sources and sends them to the Datadog API.

## Documentation

- [Architecture](architecture.md) - System design, data flow, startup sequence, and package map
- [Configuration](configuration.md) - CLI flags, YAML config file format, and environment variables
- [Metrics System](metrics-system.md) - Metrics pipeline internals: types, aggregation, and wire protocol
- [Collectors](collectors.md) - Reference for all collectors: metrics, options, tags, and modes
- [Adding a Collector](adding-a-collector.md) - Step-by-step guide for implementing a new collector
- [Library Usage](library-usage.md) - Using the datadog, metrics, and tagger packages as an imported Go library
- [Deployment](deployment.md) - Build targets, DD-WRT router, Raspberry Pi, and development setup
