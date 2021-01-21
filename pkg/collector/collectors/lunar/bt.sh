#!/bin/bash

cat << EOF | bluetoothctl
connect 00:1C:97:19:09:AD
pair 00:1C:97:19:09:AD
info 00:1C:97:19:09:AD

EOF
