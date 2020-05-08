#!/bin/sh

PID_FILE=/tmp/monitoring.pid
kill $(cat ${PID_FILE})

export DATADOG_API_KEY="fake-api-key********************"
export DATADOG_HOST_TAGS=location:home,room:living-room

exec /tmp/mnt/sda2/monitoring \
    --pid-file=${PID_FILE} \
    --datadog-host-tags=${DATADOG_HOST_TAGS} \
    --collector-datadog-client=2m \
    --collector-dnsmasq=30s  \
    --collector-dnsmasq-log=10s  \
    --collector-load=30s \
    --collector-memory=60s \
    --collector-network-arp=10s  \
    --collector-network-conntrack=10s  \
    --collector-network-statistics=10s \
    --collector-network-wireless=30s \
    --collector-tagger=2m \
    --collector-temperature=2m \
    --collector-wl=15s
