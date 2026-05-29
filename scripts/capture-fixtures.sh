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

# Traffic — a device's sumflow rollups (replace MAC with a real one).
read -r -p "MAC for a sample sumflow capture (blank to skip): " MAC
if [ -n "${MAC:-}" ]; then
  run "sel() { redis-cli --scan --pattern \"\$1\" | awk -F: '{print \$NF, \$0}' | sort -rn | head -1 | cut -d' ' -f2-; };
       emit() { k=\$(sel \"\$2\"); [ -n \"\$k\" ] && { echo \"\$1\"; redis-cli zrevrange \"\$k\" 0 -1 withscores; }; };
       emit @@DL@@ 'sumflow:${MAC}:download:*'; emit @@UL@@ 'sumflow:${MAC}:upload:*';
       emit @@LDL@@ 'sumflow:${MAC}:local:download:*'; emit @@LUL@@ 'sumflow:${MAC}:local:upload:*'" \
    > "$OUT/traffic_sumflow.txt" || true
fi

echo "Done. Review ./$OUT/, anonymize, then copy into internal/firewalla/testdata/."
