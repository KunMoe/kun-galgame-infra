#!/usr/bin/env bash
set -euo pipefail
BASE=/root/metrics
d=$(date -u +%F)
ts=$(date -u +%FT%TZ)
docker stats --no-stream --format "{{.Name}},{{.CPUPerc}},{{.MemUsage}}" | sed "s|^|$ts,|" >> "$BASE/containers-$d.csv"
read -r l1 l5 l15 _ < /proc/loadavg
memavail=$(awk "/MemAvailable/{print \$2}" /proc/meminfo)
swapfree=$(awk "/SwapFree/{print \$2}" /proc/meminfo)
echo "$ts,$l1,$l5,$l15,$memavail,$swapfree" >> "$BASE/host-$d.csv"
find "$BASE" -name "*.csv" -mtime +14 -delete
