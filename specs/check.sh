#!/usr/bin/env bash
# Check every TLA+ spec with TLC: correct configs must pass, mutation/bait
# configs must fail. Skips gracefully when no JVM is available (the Go BFS
# checker in internal/tui/loadsm enforces the same load-supersede property in
# CI without a JVM). Uses the `tlc` wrapper if present, else java + a cached
# tla2tools.jar.
set -euo pipefail
cd "$(dirname "$0")"

if command -v tlc >/dev/null 2>&1; then
  pass() { tlc --no-deadlock -c "$2" "$1"; }
  fail() { if tlc --no-deadlock -c "$2" "$1" >/dev/null 2>&1; then return 1; fi; }
else
  if ! command -v java >/dev/null 2>&1; then
    echo "no java and no tlc; skipping TLA+ spec check (Go loadsm checker covers it)"
    exit 0
  fi
  JAR="${TLA2TOOLS_JAR:-$HOME/.cache/tla2tools.jar}"
  if [ ! -f "$JAR" ]; then
    mkdir -p "$(dirname "$JAR")"
    echo "downloading tla2tools.jar…"
    curl -fsSL -o "$JAR" \
      https://github.com/tlaplus/tlaplus/releases/download/v1.8.0/tla2tools.jar \
      || { echo "download failed; skipping spec check"; exit 0; }
  fi
  tlc_run() { java -XX:+UseParallelGC -cp "$JAR" tlc2.TLC -deadlock -config "$2" "$1"; }
  pass() { tlc_run "$1" "$2"; }
  fail() { if tlc_run "$1" "$2" >/dev/null 2>&1; then return 1; fi; }
fi

echo "== specs that must hold =="
pass BlockLifecycle.tla BlockLifecycle.cfg
pass TuiLoad.tla TuiLoad.cfg

echo "== mutation configs that must fail =="
for cfg in BlockLifecycle_bug.cfg TuiLoad_none.cfg TuiLoad_viewmatch.cfg; do
  spec=$(echo "$cfg" | sed -E 's/_[a-z]+\.cfg$/.tla/')
  if fail "$spec" "$cfg"; then
    echo "  ok (violation found): $cfg"
  else
    echo "  ERROR: $cfg was expected to violate its invariant but passed"
    exit 1
  fi
done

echo "TLA+ specs OK"
