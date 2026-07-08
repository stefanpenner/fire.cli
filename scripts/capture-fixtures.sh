#!/usr/bin/env bash
#
# capture-fixtures.sh — pull raw samples from a real Firewalla box so the
# parser fixtures in internal/firewalla/testdata can be verified/refreshed.
# The schemas below are confirmed against current firmware.
#
# ⚠️  DATA HYGIENE: captured output WILL contain real MACs, IPs, hostnames,
# device names and domains. Do NOT commit it as-is. Anonymize first (see
# CLAUDE.md). This writes to a gitignored ./capture/ dir, never into testdata/.
#
# Usage: scripts/capture-fixtures.sh [ssh-destination]

set -euo pipefail

HOST="${1:-pi@fire.walla}"
OUT="capture"
mkdir -p "$OUT"
run() { ssh "$HOST" "$1"; }

echo "Capturing from $HOST into ./$OUT/ (anonymize before committing!)"

# Networks / VLANs — interface descriptors keyed by interface name.
run 'redis-cli hgetall sys:network:info' > "$OUT/network_info.txt" || true

# Devices — host:mac:<MAC> hashes, marker-delimited (one round trip).
run 'for k in $(redis-cli --scan --pattern "host:mac:*"); do
       echo "@@DEVICE@@ $k"; redis-cli hgetall "$k";
     done' > "$OUT/devices.txt" || true

# Features — the system policy hash (adblock, family, doh, vpn, qos, …).
run 'redis-cli hgetall policy:system' > "$OUT/features.txt" || true

# Alarms — newest-first active alarm hashes, marker-delimited.
run 'for id in $(redis-cli zrevrange alarm_active 0 49); do
       echo "@@ALARM@@ $id"; redis-cli hgetall "_alarm:$id";
     done' > "$OUT/alarms.txt" || true

# Rules — numeric policy:<id> hashes, marker-delimited.
run 'for k in $(redis-cli --scan --pattern "policy:*"); do
       case "$k" in policy:[0-9]*) echo "@@RULE@@ $k"; redis-cli hgetall "$k";; esac;
     done' > "$OUT/rules.txt" || true

# WAN — FireRouter routing + live connectivity + network info.
{
  echo '@@ROUTING@@'; run 'curl -s http://localhost:8837/v1/config/active'
  echo; echo '@@CONN@@'; run 'curl -s http://localhost:8837/v1/config/wan/connectivity'
  echo; echo '@@NET@@'; run 'redis-cli hgetall sys:network:info'
} > "$OUT/wan_stream.txt" || true

# Data usage — plan + per-WAN monthly rollups.
{
  echo '@@PLAN@@'; run 'redis-cli get sys:data:plan'
  run 'for k in $(redis-cli --scan --pattern "monthly:wan:data:usage:*:lastTs"); do
         u=${k#monthly:wan:data:usage:}; u=${u%:lastTs}; ts=$(redis-cli get "$k");
         echo "@@WANU@@ $u"; redis-cli get "monthly:wan:data:usage:$u:$ts"; done'
} > "$OUT/data_usage.txt" || true

# Top talkers — per-device bandwidth from sumflow rollups (box-wide).
run "for k in \$(redis-cli --scan --pattern 'sumflow:*'); do case \"\$k\" in *:local:*) continue ;; *:download:*|*:upload:*) s=\$(redis-cli zrange \"\$k\" 0 -1 withscores | awk 'NR%2==0{sum+=\$1} END{printf \"%.0f\", sum+0}'); echo \"\$k \$s\" ;; esac; done" > "$OUT/top_talkers.txt" || true

# Traffic + DNS flows — per device (replace MAC with a real one).
read -r -p "MAC for a sample sumflow/dns capture (blank to skip): " MAC
if [ -n "${MAC:-}" ]; then
  MAC_UP=$(printf '%s' "$MAC" | tr '[:lower:]' '[:upper:]')
  run "sel() { redis-cli --scan --pattern \"\$1\" | awk -F: '{print \$NF, \$0}' | sort -rn | head -1 | cut -d' ' -f2-; };
       emit() { k=\$(sel \"\$2\"); [ -n \"\$k\" ] && { echo \"\$1\"; redis-cli zrevrange \"\$k\" 0 -1 withscores; }; };
       emit @@DL@@ 'sumflow:${MAC_UP}:download:*'; emit @@UL@@ 'sumflow:${MAC_UP}:upload:*';
       emit @@LDL@@ 'sumflow:${MAC_UP}:local:download:*'; emit @@LUL@@ 'sumflow:${MAC_UP}:local:upload:*'" \
    > "$OUT/traffic_sumflow.txt" || true
  # DNS flows — the device's recent flow:dns:<MAC> zset (JSON members).
  run "redis-cli zrevrange flow:dns:${MAC_UP} 0 99" > "$OUT/dns_flows.txt" || true
fi

# Zeek dns.log — who resolved a domain today (per-3-minute gzipped logs).
read -r -p "Domain for a sample dns.log capture (blank to skip): " DOM
if [ -n "${DOM:-}" ]; then
  run "zcat /log/blog/\$(date +%F)/dns.*.log.gz 2>/dev/null | grep -iF '${DOM}' || true" \
    > "$OUT/zeek_dns.log" || true
fi

echo "Done. Review ./$OUT/, anonymize, then copy into internal/firewalla/testdata/."
