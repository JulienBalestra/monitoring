#!/bin/bash

set -eu

grep -E "collector\-[a-z0-9]" | awk '{print $1 "=0s"}' | tr '\n' ' '
