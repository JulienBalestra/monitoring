# Deployment

## Building

### Prerequisites

- Go toolchain (1.23+)

### Build Targets

```bash
make amd64     # Build for x86_64
make arm64     # Build for ARM64 (Raspberry Pi 4)
make arm       # Build for ARM (DD-WRT routers)
make re        # Clean and rebuild all architectures
```

Binaries are output to `bin/monitoring-{amd64,arm64,arm}`.

Version and commit are embedded at build time via ldflags:
- `github.com/JulienBalestra/dry/pkg/version.Version`
- `github.com/JulienBalestra/dry/pkg/version.Commit`

For specific ARM versions, set `GOARM`:
```bash
GOARM=7 make arm
```

### Other Targets

```bash
make test       # Run tests with race detector
make fmt        # Format code (go fmt)
make import     # Format imports (goimports)
make vet        # Run go vet
make lint       # Run golint
make generate   # Regenerate MAC vendor database
make clean      # Run fmt, lint, import, ineffassign, test, vet, then remove binaries
```

`clean` requires [ineffassign](https://github.com/gordonklaus/ineffassign) to be installed.

## DD-WRT Router

Reference: `setups/dd-wrt/router/`

### Prerequisites

- DD-WRT router with SSH and USB storage enabled
- dnsmasq configured with `log-queries` and `log-facility=/tmp/dnsmasq.log`

### Build and Deploy

```bash
make arm && scp bin/monitoring-arm root@192.168.1.1:/tmp/mnt/sda1/monitoring
```

### Files

Deploy to the USB storage (`/tmp/mnt/sda1/`):

- `monitoring` - the binary
- `config.yaml` - collector configuration
- `environment` - API keys
- `monitoring.sh` - startup script

### Environment File

```bash
export DATADOG_API_KEY="your-api-key"
export DATADOG_APP_KEY="your-app-key"
```

### Startup Script

The `monitoring.sh` script kills any previous instance, sources the environment, and launches the binary:

```bash
#!/bin/sh
PID_FILE=/tmp/monitoring.pid
kill $(cat ${PID_FILE})
source /tmp/mnt/sda1/environment
exec /tmp/mnt/sda1/monitoring \
    --pid-file=${PID_FILE} \
    --datadog-host-tags=os:dd-wrt \
    --log-output=/tmp/mnt/sda1/monitoring.log,datadog://zap \
    --config-file=/tmp/mnt/sda1/config.yaml
```

### Auto-Start

In DD-WRT Administration > Commands, add as a startup script:

```bash
until /tmp/mnt/sda1/monitoring.sh;do sleep 1;done
```

### Typical Collectors

```yaml
collectors:
  - name: datadog-client
  - name: dnsmasq-dhcp
  - name: dnsmasq-log
  - name: dnsmasq-queries
  - name: golang
  - name: load
  - name: memory
  - name: network-arp
  - name: network-conntrack
  - name: network-statistics
  - name: network-wireless
  - name: tagger
  - name: temperature-dd-wrt
  - name: uptime
  - name: wl
```

## DD-WRT Repeater Bridge

Reference: `setups/dd-wrt/repeater-bridge/`

Same deployment process as the router, with a reduced collector set (no dnsmasq collectors since DHCP runs on the main router):

```yaml
collectors:
  - name: datadog-client
  - name: golang
  - name: load
  - name: memory
  - name: network-arp
  - name: network-conntrack
  - name: network-statistics
  - name: network-wireless
  - name: tagger
  - name: temperature-dd-wrt
  - name: uptime
  - name: wl
```

## Raspberry Pi with WireGuard

Reference: `setups/raspberry-pi-wireguard/`

### Build and Deploy

```bash
make arm64
sudo cp bin/monitoring-arm64 /usr/local/bin/monitoring
sudo mkdir -p /etc/monitoring
sudo cp setups/raspberry-pi-wireguard/config.yaml /etc/monitoring/config.yaml
sudo cp setups/raspberry-pi-wireguard/environment /etc/monitoring/environment
sudo cp setups/raspberry-pi-wireguard/monitoring.service /etc/systemd/system/monitoring.service
```

### Environment File

```bash
DATADOG_API_KEY="your-api-key"
DATADOG_APP_KEY="your-app-key"
```

### Systemd Service

The service file (`monitoring.service`) runs the binary with auto-restart:

```ini
[Unit]
Description=monitoring
Documentation=https://github.com/JulienBalestra/monitoring

[Service]
Environment=DATADOG_API_KEY=
Environment=DATADOG_APP_KEY=
EnvironmentFile=/etc/monitoring/environment
ExecStart=/usr/local/bin/monitoring \
    --datadog-host-tags=os:2004.1 \
    --pid-file=/run/monitoring.pid \
    --log-output=stdout,datadog://zap \
    --config-file=/etc/monitoring/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Enable and Start

```bash
sudo systemctl enable --now monitoring
```

### Typical Collectors

```yaml
collectors:
  - name: datadog-client
  - name: load
  - name: golang
  - name: memory
  - name: network-arp
  - name: network-statistics
  - name: tagger
  - name: temperature-raspberry-pi
  - name: uptime
  - name: wireguard
```

## Development

Reference: `setups/dev/`

For local development, use a minimal config:

```yaml
collectors:
  - name: etcd
```

Run directly:

```bash
make amd64 && bin/monitoring-amd64 \
    --config-file=setups/dev/config.yaml \
    --log-output=stdout
```

## Secrets

- Never commit real API keys. The `environment` files in `setups/` contain placeholder values.
- Use environment files or env vars to pass `DATADOG_API_KEY` and `DATADOG_APP_KEY`.

## Logging

Output paths are configured via `--log-output` (comma-separated):

| Output | Description |
|--------|-------------|
| `stdout` | Standard output |
| `datadog://zap` | Forward logs to the Datadog Logs API |
| `/path/to/file` | Write logs to a file |

Log levels: `debug`, `info`, `warn`, `error`, `dpanic`, `panic`, `fatal`

## Datadog Dashboards

Pre-built dashboard JSON files are available in `setups/dd-wrt/datadog-dashboards/`. These can be imported into Datadog for visualization.
