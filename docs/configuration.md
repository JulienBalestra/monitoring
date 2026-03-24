# Configuration

## CLI Flags

| Flag | Short | Default | Env Var | Description |
|------|-------|---------|---------|-------------|
| `--hostname` | | `os.Hostname()` (lowered) | | Datadog host tag |
| `--datadog-api-key` | `-i` | `""` | `DATADOG_API_KEY` | Datadog API key |
| `--datadog-app-key` | `-p` | `""` | `DATADOG_APP_KEY` | Datadog APP key |
| `--datadog-client-send-interval` | | `35s` | | Batch send interval (minimum `5s`) |
| `--datadog-host-tags` | | `nil` | | Additional host tags (comma-separated) |
| `--config-file` | `-c` | `/etc/monitoring/config.yaml` | | Path to YAML configuration file |
| `--log-level` | | `info` | | Log level: debug, info, warn, error, dpanic, panic, fatal |
| `--log-output` | | `stdout,datadog://zap` | | Log output paths |
| `--timezone` | | system local | | Application timezone (e.g. `UTC`, `Europe/Paris`) |
| `--pid-file` | | `/tmp/monitoring.pid` | | PID file path |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DATADOG_API_KEY` | Datadog API key (used if `--datadog-api-key` flag is empty) |
| `DATADOG_APP_KEY` | Datadog APP key (used if `--datadog-app-key` flag is empty) |

## Configuration File (YAML)

The config file specifies which collectors to run and their settings. A collector not listed in the file is not started.

### Structure

```yaml
collectors:
  - name: <collector-name>       # required: must match a registered collector
    interval: <duration>          # optional: overrides collector's default interval
    options:                      # optional: merged with collector's default options
      key: value
    tags:                         # optional: merged with collector's default tags
      - "key:value"
```

### Example

```yaml
collectors:
  - name: load
  - name: memory
  - name: uptime
  - name: golang
  - name: network-arp
  - name: network-statistics
  - name: tagger
  - name: datadog-client
  - name: wireguard
  - name: temperature-raspberry-pi
```

### Multiple Instances

A collector can appear multiple times in the config for different targets:

```yaml
collectors:
  - name: ping
    options:
      target: "1.1.1.1"
    tags:
      - "target:cloudflare"
  - name: ping
    options:
      target: "8.8.8.8"
    tags:
      - "target:google"
```

### Generated Default Config

The root `config.yaml` is a symlink to `pkg/collector/catalog/fixtures/gen-collectors.yaml`, which contains all collectors with their default options, intervals, and tags.

The `GenerateCollectorConfigFile()` function in `pkg/collector/catalog/catalog.go` can regenerate this fixture programmatically. Note: `make generate` regenerates the MAC vendor database, not the config fixture.

## Setup Examples

Pre-built configurations are available in `setups/`:

- **`setups/dd-wrt/router/`** - DD-WRT router with dnsmasq, network, and temperature collectors
- **`setups/dd-wrt/repeater-bridge/`** - DD-WRT in repeater bridge mode
- **`setups/raspberry-pi-wireguard/`** - Raspberry Pi with WireGuard, systemd service
- **`setups/dev/`** - Minimal development configuration
