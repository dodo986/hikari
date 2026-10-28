#!/usr/bin/env bash
# Peak-ish resident memory of one UI against the demo daemon: PSS summed over
# the whole process tree, including WebKit's helper processes, after idling.
set -euo pipefail
here="$(cd "$(dirname "$0")/../.." && pwd)"
runtime="${HIKARI_MEM_RUNTIME:-/tmp/hikari-mem}"
idle="${HIKARI_MEM_IDLE:-60}"
export XDG_RUNTIME_DIR="$runtime"
export HIKARI_DEV_RUNTIME="$runtime"

children_of() {
  local parent="$1"
  awk -v p="$parent" '$4 == p { print $1 }' /proc/[0-9]*/stat 2>/dev/null | tr -d '()' || true
}

tree_pids() {
  local queue=("$1") seen=() pid child
  while [ ${#queue[@]} -gt 0 ]; do
    pid="${queue[0]}"
    queue=("${queue[@]:1}")
    case " ${seen[*]-} " in *" $pid "*) continue ;; esac
    seen+=("$pid")
    for child in $(children_of "$pid"); do queue+=("$child"); done
  done
  printf '%s\n' "${seen[@]}"
}

pss_kb() {
  local total=0 pid value
  for pid in $(tree_pids "$1"); do
    value=$(awk '/^Pss:/ { print $2 }' "/proc/$pid/smaps_rollup" 2>/dev/null || true)
    [ -n "${value:-}" ] && total=$((total + value))
  done
  printf '%s\n' "$total"
}

wait_for_window() {
  local tries=0
  while [ $tries -lt 100 ]; do
    if niri msg windows 2>/dev/null | grep -qi "$1"; then return 0; fi
    sleep 0.3
    tries=$((tries + 1))
  done
  return 1
}

start_daemon() {
  mkdir -p "$runtime"
  ( cd "$here" && go build -o "$runtime/hikarid" ./cmd/hikarid )
  "$runtime/hikarid" --demo >"$runtime/daemon.log" 2>&1 &
  daemon_pid=$!
  sleep 2
}

stop_daemon() { kill "$daemon_pid" 2>/dev/null || true; }

measure() {
  local name="$1" needle="$2"
  shift 2
  "$@" >"$runtime/$name.log" 2>&1 &
  local pid=$!
  if ! wait_for_window "$needle"; then
    echo "$name: no window appeared" >&2
    kill "$pid" 2>/dev/null || true
    return 1
  fi
  sleep "$idle"
  local kb
  kb=$(pss_kb "$pid")
  echo "$name $kb"
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  sleep 1
}

mkdir -p "$runtime"
start_daemon
trap 'stop_daemon' EXIT

new_kb=$(measure "new-ui" "Hikari" "$here/frontend/build/hikari-ui" | awk '{ print $2 }')
old_kb=$(measure "quickshell" "hikari" qs -n -c hikari | awk '{ print $2 }')

[ -n "${new_kb:-}" ] && [ -n "${old_kb:-}" ] || { echo "measurement failed" >&2; exit 1; }

awk -v new="$new_kb" -v old="$old_kb" 'BEGIN {
  printf "new      %8.1f MiB\n", new / 1024
  printf "previous %8.1f MiB\n", old / 1024
  printf "ratio    %8.2fx\n", new / old
}'
