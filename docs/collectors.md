# Collectors Reference

All collectors implement the `Collector` interface defined in `pkg/collector/interface.go` and are registered in the catalog at `pkg/collector/catalog/catalog.go`.

## Quick Reference

| Name | Category | Mode | Default Interval | Platform |
|------|----------|------|-----------------|----------|
| `memory` | System | periodic | 60s | Linux |
| `load` | System | periodic | 15s | Linux |
| `uptime` | System | periodic | 5m | Linux |
| `golang` | System | periodic | 2m | any |
| `network-arp` | Network | periodic | 10s | Linux |
| `network-conntrack` | Network | daemon | 10s | Linux |
| `network-statistics` | Network | periodic | 10s | Linux |
| `network-wireless` | Network | periodic | 10s | Linux |
| `ping` | Network | periodic | 1m | any |
| `wl` | Network | periodic | 15s | DD-WRT |
| `dnsmasq-queries` | DNS/DHCP | periodic | 30s | any |
| `dnsmasq-log` | DNS/DHCP | daemon | 10s | any |
| `dnsmasq-dhcp` | DNS/DHCP | periodic | 30s | any |
| `wireguard` | WireGuard | periodic | 10s | Linux |
| `wireguard-stun-peer-etcd` | WireGuard | periodic | 30s | any |
| `wireguard-stun-registry-etcd` | WireGuard | periodic | 30s | any |
| `shelly` | IoT | periodic | 5s | any |
| `freebox` | IoT | periodic | 10s | any |
| `google-home` | IoT | periodic | 30s | any |
| `bluetooth` | IoT | daemon | 30s | Linux |
| `acaia-lunar` | IoT | daemon | 10s | Linux |
| `temperature-dd-wrt` | Temperature | periodic | 2m | DD-WRT |
| `temperature-raspberry-pi` | Temperature | periodic | 2m | Raspberry Pi |
| `prometheus` | Services | periodic | 30s | any |
| `coredns` | Services | periodic | 30s | any |
| `etcd` | Services | periodic | 30s | any |
| `http` | Services | periodic | 30s | any |
| `tagger` | Meta | periodic | 2m | any |
| `datadog-client` | Meta | periodic | 2m | any |

## System

### memory

- **Source**: `pkg/collector/collectors/memory/memory.go`
- **Mode**: periodic
- **Default Interval**: 60s
- **Options**: none
- **Default Tags**: `collector:memory`
- **Platform**: Linux (syscall.Sysinfo)

| Metric | Type | Description |
|--------|------|-------------|
| `memory.ram.total` | gauge | Total RAM in bytes |
| `memory.ram.free` | gauge | Free RAM in bytes |
| `memory.ram.shared` | gauge | Shared RAM in bytes |
| `memory.ram.buffer` | gauge | Buffer RAM in bytes |
| `memory.swap.total` | gauge | Total swap in bytes |
| `memory.swap.free` | gauge | Free swap in bytes |

All metrics use GaugeDeviation (only sent when value changes).

---

### load

- **Source**: `pkg/collector/collectors/load/load.go`
- **Mode**: periodic
- **Default Interval**: 15s
- **Options**: none
- **Default Tags**: `collector:load`
- **Platform**: Linux (syscall.Sysinfo)

| Metric | Type | Description |
|--------|------|-------------|
| `load.1` | gauge | 1-minute load average |
| `load.5` | gauge | 5-minute load average |
| `load.15` | gauge | 15-minute load average |

---

### uptime

- **Source**: `pkg/collector/collectors/uptime/uptime.go`
- **Mode**: periodic
- **Default Interval**: 5m
- **Options**: none
- **Default Tags**: `collector:uptime`
- **Platform**: Linux (syscall.Sysinfo)

| Metric | Type | Description |
|--------|------|-------------|
| `up.time` | gauge | System uptime in seconds |

---

### golang

- **Source**: `pkg/collector/collectors/golang/golang.go`
- **Mode**: periodic
- **Default Interval**: 2m
- **Options**: none
- **Default Tags**: `collector:golang`
- **Platform**: any

| Metric | Type | Description |
|--------|------|-------------|
| `golang.runtime.goroutines` | gauge | Number of goroutines |
| `golang.heap.alloc` | gauge | Heap allocation in bytes |

---

## Network

### network-arp

- **Source**: `pkg/collector/collectors/network/arp/arp.go`
- **Mode**: periodic
- **Default Interval**: 10s
- **Default Tags**: `collector:network-arp`
- **Platform**: Linux

| Option | Default | Description |
|--------|---------|-------------|
| `arp-file` | `/proc/self/net/arp` | Path to ARP table |

| Metric | Type | Description |
|--------|------|-------------|
| `network.arp` | gauge | ARP entry (one per IP/MAC pair) |

**Dynamic Tags**: `mac`, `ip`, `device`, `vendor` (via macvendor lookup), `lease` (from tagger/DHCP).

Enriches the tagger with IP and MAC entity tags for use by other collectors.

---

### network-conntrack

- **Source**: `pkg/collector/collectors/network/conntrack/conntrack.go`
- **Mode**: daemon
- **Default Interval**: 10s
- **Default Tags**: `collector:network-conntrack`
- **Platform**: Linux

| Option | Default | Description |
|--------|---------|-------------|
| `conntrack-file` | `/proc/net/ip_conntrack` | Path to conntrack table |

| Metric | Type | Description |
|--------|------|-------------|
| `network.conntrack.entries` | gauge | Connection tracking entries |

**Dynamic Tags**: `protocol`, `dport`, `ip`, `state`, `lease`, `device`.

Parses TCP/UDP/ICMP connection states (ESTABLISHED, UNREPLIED, REPLIED) with port ranges.

---

### network-statistics

- **Source**: `pkg/collector/collectors/network/statistics/statistics.go`
- **Mode**: periodic
- **Default Interval**: 10s
- **Default Tags**: `collector:network-statistics`
- **Platform**: Linux

| Option | Default | Description |
|--------|---------|-------------|
| `sys-class-net-path` | `/sys/class/net/` | Path to network interface sysfs |

| Metric | Type | Description |
|--------|------|-------------|
| `network.statistics.*` | count | Dynamic metrics from `/sys/class/net/*/statistics/` files |

Reads all files in each interface's statistics directory (e.g., `rx_bytes`, `tx_bytes`, `rx_packets`, `tx_packets`, `rx_errors`, etc.). Metric names are `network.statistics.<filename>`.

---

### network-wireless

- **Source**: `pkg/collector/collectors/network/wireless/wireless.go`
- **Mode**: periodic
- **Default Interval**: 10s
- **Default Tags**: `collector:network-wireless`
- **Platform**: Linux

| Option | Default | Description |
|--------|---------|-------------|
| `sys-class-net-path` | `/sys/class/net/` | Path to network sysfs |
| `proc-net-wireless-file` | `/proc/net/wireless` | Path to wireless info |

| Metric | Type | Description |
|--------|------|-------------|
| `network.wireless.noise` | gauge | Wireless noise level |
| `network.wireless.discard.retry` | gauge | Wireless discard/retry count |

---

### ping

- **Source**: `pkg/collector/collectors/ping/ping.go`
- **Mode**: periodic
- **Default Interval**: 1m
- **Default Tags**: `collector:ping`
- **Platform**: any (requires ping command)

| Option | Default | Description |
|--------|---------|-------------|
| `target` | `1.1.1.1` | IP address to ping |
| `timeout-sec` | `2` | Ping timeout in seconds |

| Metric | Type | Description |
|--------|------|-------------|
| `latency.icmp` | gauge | ICMP round-trip latency |

**Dynamic Tags**: `ip`, `target`.

---

### wl

- **Source**: `pkg/collector/collectors/wl/wl.go`
- **Mode**: periodic
- **Default Interval**: 15s
- **Default Tags**: `collector:wl`
- **Platform**: DD-WRT (Broadcom wireless)

| Option | Default | Description |
|--------|---------|-------------|
| `wl-exe` | `/usr/sbin/wl` | Path to wl command |
| `proc-net-wireless-path` | `/proc/net/wireless` | Path to wireless info |

| Metric | Type | Description |
|--------|------|-------------|
| `network.wireless.rssi.dbm` | gauge | Wireless RSSI in dBm |

---

## DNS / DHCP (dnsmasq)

### dnsmasq-queries

- **Source**: `pkg/collector/collectors/dnsmasq/dnsqueries/queries.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:dnsmasq-queries`
- **Platform**: any (requires dnsmasq)

| Option | Default | Description |
|--------|---------|-------------|
| `address` | `127.0.0.1:53` | dnsmasq DNS address |

| Metric | Type | Description |
|--------|------|-------------|
| `dnsmasq.dns.cache.hit` | count | Cache hits |
| `dnsmasq.dns.cache.miss` | count | Cache misses |
| `dnsmasq.dns.cache.eviction` | count | Cache evictions |
| `dnsmasq.dns.cache.insertion` | count | Cache insertions |
| `dnsmasq.dns.cache.size` | gauge | Cache size |

Queries dnsmasq via DNS CHAOS TXT records on the bind interface.

---

### dnsmasq-log

- **Source**: `pkg/collector/collectors/dnsmasq/dnslogs/dnslog.go`
- **Mode**: daemon
- **Default Interval**: 10s
- **Default Tags**: `collector:dnsmasq-log`
- **Platform**: any (requires dnsmasq with `log-queries`)

| Option | Default | Description |
|--------|---------|-------------|
| `log-facility-file` | `/tmp/dnsmasq.log` | dnsmasq log file path |

| Metric | Type | Description |
|--------|------|-------------|
| `dnsmasq.dns.query` | count (Incr) | DNS query count |

**Dynamic Tags**: `domain`, `type`, `ip`, `lease`.

Tails the dnsmasq log file and parses query lines.

---

### dnsmasq-dhcp

- **Source**: `pkg/collector/collectors/dnsmasq/dhcp/dhcp.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:dnsmasq-dhcp`
- **Platform**: any (requires dnsmasq DHCP)

| Option | Default | Description |
|--------|---------|-------------|
| `leases-file` | `/tmp/dnsmasq.leases` | DHCP leases file path |

| Metric | Type | Description |
|--------|------|-------------|
| `dnsmasq.dhcp.lease` | gauge | DHCP lease entries |

Enriches the tagger with lease names for MAC/IP entities, enabling cross-collector tag lookup.

---

## WireGuard

### wireguard

- **Source**: `pkg/collector/collectors/wireguard/wireguard.go`
- **Mode**: periodic
- **Default Interval**: 10s
- **Default Tags**: `collector:wireguard`
- **Platform**: Linux (requires WireGuard)

No configurable options.

| Metric | Type | Description |
|--------|------|-------------|
| `wireguard.active` | gauge | Number of active peers |
| `wireguard.inactive` | gauge | Number of inactive peers |
| `wireguard.transfer.received` | count | Bytes received per peer |
| `wireguard.transfer.sent` | count | Bytes sent per peer |
| `wireguard.handshake.age` | gauge | Seconds since last handshake per peer |

**Dynamic Tags**: `device`, `pub-key-sha1`, `pub-key-sha1-7`, `allowed-ips`, `endpoint`, `ip`, `port`, `wg-active`.

Uses the wgctrl library to query WireGuard interfaces.

---

### wireguard-stun-peer-etcd

- **Source**: `pkg/collector/collectors/wireguard-stun/peer/etcd/etcd.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:wireguard-stun-peer-etcd`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `exporter-url` | `http://127.0.0.1:8989/metrics` | Prometheus endpoint |
| `wireguard_stun_peers` | `wireguard_stun.peers` | Metric name mapping |
| `wireguard_stun_peer_etcd_updates` | `wireguard_stun.peer.etcd.updates` | Metric name mapping |
| `wireguard_stun_etcd_conn_state` | `wireguard_stun.etcd.conn.state` | Metric name mapping |
| `go_memstats_heap_alloc_bytes` | `golang.heap.alloc` | Metric name mapping |
| `go_goroutines` | `golang.runtime.goroutines` | Metric name mapping |

Wraps the Prometheus exporter to scrape WireGuard STUN peer metrics from etcd.

---

### wireguard-stun-registry-etcd

- **Source**: `pkg/collector/collectors/wireguard-stun/registry/etcd/etcd.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:wireguard-stun-registry-etcd`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `exporter-url` | `http://127.0.0.1:8989/metrics` | Prometheus endpoint |
| `wireguard_stun_peers` | `wireguard_stun.peers` | Metric name mapping |
| `wireguard_stun_registry_etcd_txn` | `wireguard_stun.registry.etcd.txn` | Metric name mapping |
| `wireguard_stun_registry_etcd_update_triggers` | `wireguard_stun.registry.etcd.updates` | Metric name mapping |
| `wireguard_stun_etcd_conn_state` | `wireguard_stun.etcd.conn.state` | Metric name mapping |
| `go_memstats_heap_alloc_bytes` | `golang.heap.alloc` | Metric name mapping |
| `go_goroutines` | `golang.runtime.goroutines` | Metric name mapping |

Wraps the Prometheus exporter to scrape WireGuard STUN registry metrics from etcd.

---

## IoT / Devices

### shelly

- **Source**: `pkg/collector/collectors/shelly/shelly.go`
- **Mode**: periodic
- **Default Interval**: 5s
- **Default Tags**: `collector:shelly`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `endpoint` | `http://192.168.1.2` | Shelly device HTTP endpoint |

| Metric | Type | Description |
|--------|------|-------------|
| `temperature.celsius` | gauge | Temperature sensor reading |
| `network.wireless.rssi.dbm` | gauge | WiFi signal strength |
| `memory.ram.free` | gauge | Free RAM on device |
| `memory.ram.total` | gauge | Total RAM on device |
| `filesystem.free` | gauge | Free filesystem space |
| `filesystem.size` | gauge | Total filesystem size |
| `up.time` | gauge | Device uptime |
| `power.current` | gauge | Current power draw |
| `power.total` | gauge | Total power consumed |
| `power.on` | gauge | Relay on/off state |

**Dynamic Tags**: `ip`, `mac`, `shelly-model`, `sensor`, `ssid`, `meter`, `relay`.

---

### freebox

- **Source**: `pkg/collector/collectors/freebox/freebox.go`
- **Mode**: periodic
- **Default Interval**: 10s
- **Default Tags**: `collector:freebox`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `method` | `GET` | HTTP method |
| `url` | `http://mafreebox.freebox.fr/api_version` | Freebox API endpoint |

| Metric | Type | Description |
|--------|------|-------------|
| `latency.http` | gauge | HTTP request latency |

Wraps the HTTP collector to monitor a Freebox router.

---

### google-home

- **Source**: `pkg/collector/collectors/google_home/google_home.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:google-home`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| (requires `ip` option) | | Google Home device IP |

| Metric | Type | Description |
|--------|------|-------------|
| `up.time` | gauge | Device uptime |
| `network.wireless.rssi.dbm` | gauge | WiFi signal strength |
| `network.wireless.noise` | gauge | WiFi noise level |
| `google.home.has_update` | gauge | Update available (0/1) |
| `google.home.connected` | gauge | Connected state (0/1) |
| `google.home.setup_state` | gauge | Setup state |

---

### bluetooth (WIP)

- **Source**: `pkg/collector/collectors/bluetooth/bluetooth.go`
- **Mode**: daemon
- **Default Interval**: 30s
- **Default Tags**: `collector:bluetooth`
- **Platform**: Linux (D-Bus / BlueZ)

| Metric | Type | Description |
|--------|------|-------------|
| `bluetooth.rssi.dbm` | gauge | Bluetooth RSSI |
| `bluetooth.devices` | gauge | Number of discovered devices |

---

### acaia-lunar (WIP)

- **Source**: `pkg/collector/collectors/lunar/lunar.go`
- **Mode**: daemon
- **Default Interval**: 10s
- **Default Tags**: `collector:acaia-lunar`
- **Platform**: Linux (D-Bus / BlueZ BLE)

| Option | Default | Description |
|--------|---------|-------------|
| `lunar-service-uuid` | `00002a80-0000-1000-8000-00805f9b34fb` | BLE service UUID |
| `lunar-uuid` | `00001820-0000-1000-8000-00805f9b34fb` | BLE device UUID |

Acaia Lunar coffee scale integration via Bluetooth Low Energy.

---

## Temperature

### temperature-dd-wrt

- **Source**: `pkg/collector/collectors/temperature/ddwrt/temperature.go`
- **Mode**: periodic
- **Default Interval**: 2m
- **Default Tags**: `collector:temperature-dd-wrt`
- **Platform**: DD-WRT

| Option | Default | Description |
|--------|---------|-------------|
| `temperature-file` | `/proc/dmu/temperature` | Temperature sensor file |

| Metric | Type | Description |
|--------|------|-------------|
| `temperature.celsius` | gauge | Hardware temperature |

---

### temperature-raspberry-pi

- **Source**: `pkg/collector/collectors/temperature/raspberrypi/temperature.go`
- **Mode**: periodic
- **Default Interval**: 2m
- **Default Tags**: `collector:temperature-raspberry-pi`
- **Platform**: Raspberry Pi

| Option | Default | Description |
|--------|---------|-------------|
| `temperature-file` | `/sys/class/thermal/thermal_zone0/temp` | Thermal zone file |

| Metric | Type | Description |
|--------|------|-------------|
| `temperature.celsius` | gauge | CPU temperature |

---

## Services

### prometheus

- **Source**: `pkg/collector/collectors/prometheus/exporter/exporter.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:prometheus`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `exporter-url` | (required) | Prometheus metrics endpoint URL |
| `go_memstats_heap_alloc_bytes` | `golang.heap.alloc` | Metric name mapping |
| `go_goroutines` | `golang.runtime.goroutines` | Metric name mapping |

Options map Prometheus metric names (keys) to Datadog metric names (values). Only mapped metrics are collected. Handles Counter, Gauge, Summary, and Histogram types from Prometheus exposition format.

---

### coredns

- **Source**: `pkg/collector/collectors/coredns/coredns.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:coredns`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `exporter-url` | `http://127.0.0.1:9153/metrics` | CoreDNS metrics endpoint |
| `coredns_dns_requests_total` | `coredns.dns.requests` | Metric name mapping |
| `coredns_dns_responses_total` | `coredns.dns.responses` | Metric name mapping |
| `go_memstats_heap_alloc_bytes` | `golang.heap.alloc` | Metric name mapping |
| `go_goroutines` | `golang.runtime.goroutines` | Metric name mapping |

Wraps the Prometheus exporter for CoreDNS.

---

### etcd

- **Source**: `pkg/collector/collectors/etcd/etcd.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:etcd`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `exporter-url` | `http://127.0.0.1:2379/metrics` | etcd metrics endpoint |
| `etcd_mvcc_db_total_size_in_bytes` | `etcd.db.total.size` | DB total size |
| `etcd_mvcc_db_total_size_in_use_in_bytes` | `etcd.db.use.size` | DB used size |
| `etcd_disk_wal_write_bytes_total` | `etcd.wall.writes` | WAL write bytes |
| `etcd_debugging_mvcc_total_put_size_in_bytes` | `etcd.put.size` | Put size bytes |
| `grpc_server_handled_total` | `etcd.grpc.calls` | gRPC calls |
| `etcd_debugging_mvcc_keys_total` | `etcd.keys` | Total keys |
| `etcd_debugging_mvcc_watch_stream_total` | `etcd.watch.streams` | Watch streams |
| `etcd_debugging_mvcc_watcher_total` | `etcd.watch.watchers` | Watchers |
| `etcd_debugging_mvcc_put_total` | `etcd.puts` | Total puts |
| `go_memstats_heap_alloc_bytes` | `golang.heap.alloc` | Heap allocation |
| `go_goroutines` | `golang.runtime.goroutines` | Goroutines |

Wraps the Prometheus exporter for etcd.

---

### http

- **Source**: `pkg/collector/collectors/http/http.go`
- **Mode**: periodic
- **Default Interval**: 30s
- **Default Tags**: `collector:http`
- **Platform**: any

| Option | Default | Description |
|--------|---------|-------------|
| `method` | `GET` | HTTP method |
| `url` | (required) | Target URL |

| Metric | Type | Description |
|--------|------|-------------|
| `latency.http` | gauge | HTTP request latency |

**Dynamic Tags**: `code`, `url`, `host-target`, `method`, `path`, `port`, `ip`, `scheme`.

---

## Meta

### tagger

- **Source**: `pkg/collector/collectors/tagger/tagger.go`
- **Mode**: periodic
- **Default Interval**: 2m
- **Default Tags**: `collector:tagger`
- **Platform**: any

| Metric | Type | Description |
|--------|------|-------------|
| `tagger.entities` | gauge | Number of entities in the tag store |
| `tagger.keys` | gauge | Number of tag keys |
| `tagger.tags` | gauge | Total number of tags |

---

### datadog-client

- **Source**: `pkg/collector/collectors/datadog/client.go`
- **Mode**: periodic
- **Default Interval**: 2m
- **Default Tags**: `collector:datadog-client`
- **Platform**: any

| Metric | Type | Description |
|--------|------|-------------|
| `client.sent.metrics.bytes` | count | Compressed bytes sent to Datadog API |
| `client.sent.metrics.series` | count | Number of series sent |
| `client.metrics.errors` | count | Send failures |
| `client.metrics.store.aggregations` | count | Series merged during aggregation |
| `client.sent.logs.bytes` | count | Log bytes sent |
| `client.logs.errors` | count | Log send failures |

Reports internal Datadog client statistics for self-monitoring.
