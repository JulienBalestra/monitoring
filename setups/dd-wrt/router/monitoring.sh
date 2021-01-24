#!/bin/sh

PID_FILE=/tmp/monitoring.pid
kill $(cat ${PID_FILE})

source /tmp/mnt/sda1/environment

exec /tmp/mnt/sda1/monitoring \
    --pid-file=${PID_FILE} \
    --datadog-host-tags=os:dd-wrt \
    --log-output=/tmp/mnt/sda1/monitoring.log,datadog://zap \
    --config-file=/tmp/mnt/sda1/config.yaml
