#!/usr/bin/env bash
# coverage_gate.sh — per-plugin statement coverage gate & report.
#
# The GA/Beta maturity gates are defined in docs/architecture/plugin-architecture.md:
# every GA (and GA-ready Beta) plugin must keep module statement coverage >= 50%.
# This script is the enforcement point:
#
#   gate <plugin> [threshold]   run tests, fail (exit 1) below the threshold.
#                               Plugins outside GATED_PLUGINS pass unless an
#                               explicit threshold is given.
#   report [--all] [plugin...]  print a coverage table. "--all" covers every
#                               plugin module (source for the doc table).
#
# Keep GATED_PLUGINS in sync with docs/architecture/plugin-architecture.md.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGINS_DIR="$ROOT/plugins"

# GA + GA-ready Beta plugins gated at GATE_THRESHOLD (sync with docs/architecture/plugin-architecture.md).
GATED_PLUGINS=(
  # GA
  auth cache gateway health metrics rpc tracing websocket
  # GA-ready Beta (technical gates met, pending production case)
  actor alert audit bloomfilter db discovery gameserver
  loadbalancer rbac scheduler upload
)
GATE_THRESHOLD=50

usage() {
  echo "usage: $0 gate <plugin> [threshold]"
  echo "       $0 report [--all] [plugin ...]"
}

list_all_plugins() {
  for dir in "$PLUGINS_DIR"/*/; do
    [ -f "$dir/go.mod" ] || continue
    basename "$dir"
  done
}

is_gated() {
  local name="$1"
  for p in "${GATED_PLUGINS[@]}"; do
    [ "$p" = "$name" ] && return 0
  done
  return 1
}

threshold_for() {
  local name="$1"
  is_gated "$name" && echo "$GATE_THRESHOLD" || echo 0
}

# total_coverage <plugin> -> prints "<coverage> <exitcode>" where coverage is a
# float like "53.9" (0.0 when no coverage data). Runs `go test` in the module.
total_coverage() {
  local plugin="$1"
  local module_dir="$PLUGINS_DIR/$plugin"
  local profile
  profile="$(mktemp)"
  trap 'rm -f "$profile"' RETURN

  if (cd "$module_dir" && go test -count=1 -coverprofile="$profile" ./... >/dev/null 2>&1); then
    :
  else
    echo "0.0 1"
    return
  fi

  local total
  total="$(go tool cover -func="$profile" 2>/dev/null | awk '/^total:/ {print $3}')"
  if [ -z "$total" ]; then
    # Module built but produced no coverage data (no test files).
    echo "0.0 0"
    return
  fi
  echo "${total%\%} 0"
}

cmd_gate() {
  local plugin="${1:-}"
  [ -z "$plugin" ] && { usage; return 2; }
  local module_dir="$PLUGINS_DIR/$plugin"
  if [ ! -f "$module_dir/go.mod" ]; then
    echo "coverage gate: unknown plugin '$plugin'" >&2
    return 2
  fi

  local threshold
  threshold="${2:-$(threshold_for "$plugin")}"
  if ! is_gated "$plugin" && [ "$#" -lt 2 ]; then
    echo "coverage gate: $plugin is not gated (no threshold configured)" >&2
    return 0
  fi

  local result cov code
  result="$(total_coverage "$plugin")"
  cov="${result% *}"
  code="${result#* }"
  if [ "$code" != "0" ]; then
    echo "coverage gate: $plugin FAILED (go test exited non-zero; fix failing tests before measuring coverage)" >&2
    return 1
  fi

  local pass
  pass="$(awk -v c="$cov" -v t="$threshold" 'BEGIN { print (c+0 >= t+0) ? 1 : 0 }')"
  printf 'coverage gate: %-14s %.1f%% (threshold %.0f%%) -> %s\n' \
    "$plugin" "$cov" "$threshold" "$([ "$pass" = "1" ] && echo PASS || echo FAIL)"
  [ "$pass" = "1" ]
}

cmd_report() {
  local plugins=()
  local all=0
  local arg
  for arg in "$@"; do
    case "$arg" in
      --all) all=1 ;;
      -h|--help) usage; return 0 ;;
      *) plugins+=("$arg") ;;
    esac
  done
  if [ "$all" = "1" ]; then
    mapfile -t plugins < <(list_all_plugins)
  elif [ "${#plugins[@]}" -eq 0 ]; then
    plugins=("${GATED_PLUGINS[@]}")
  fi

  printf '%-16s %10s %8s %s\n' "plugin" "coverage" "gate" "status"
  printf '%-16s %10s %8s %s\n' "------" "--------" "----" "------"

  local failed=0
  local plugin result cov threshold pass
  for plugin in "${plugins[@]}"; do
    if [ ! -f "$PLUGINS_DIR/$plugin/go.mod" ]; then
      printf '%-16s %10s %8s %s\n' "$plugin" "-" "-" "unknown plugin"
      failed=1
      continue
    fi
    result="$(total_coverage "$plugin")"
    cov="${result% *}"
    threshold="$(threshold_for "$plugin")"
    if [ "${result#* }" != "0" ]; then
      printf '%-16s %10s %8s %s\n' "$plugin" "ERROR" ">=$threshold%" "go test failed"
      failed=1
      continue
    fi
    pass="$(awk -v c="$cov" -v t="$threshold" 'BEGIN { print (c+0 >= t+0) ? 1 : 0 }')"
    printf '%-16s %9s%% %7s%% %s\n' "$plugin" "$cov" "$threshold" \
      "$([ "$pass" = "1" ] && echo PASS || echo FAIL)"
    [ "$pass" = "1" ] || failed=1
  done
  return "$failed"
}

case "${1:-}" in
  gate) shift; cmd_gate "$@" ;;
  report) shift; cmd_report "$@" ;;
  -h|--help|"") usage ;;
  *) usage; exit 2 ;;
esac
