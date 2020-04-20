#!/bin/sh
kill $(cat /tmp/metrics.pid)

export DATADOG_API_KEY="fake-api-key********************"
export DATADOG_HOST_TAGS=location:home,room:living-room

exec /jffs/bin/metrics --pid-file=/tmp/metrics.pid \
    --collector-datadog-client                  90s     \
    --collector-dnsmasq                         30s     \
    --collector-load                            15s     \
    --collector-memory                          30s     \
    --collector-network-arp                     30s     \
    --collector-network-conntrack               30s     \
    --collector-network-statistics              10s     \
    --collector-tagger                          90s     \
    --collector-temperature                     60s     \
    --datadog-host-tags=${DATADOG_HOST_TAGS}
