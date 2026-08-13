#!/usr/bin/env bash
set -euo pipefail

image=${1:?usage: benchmark.sh IMAGE [RUNS]}
runs=${2:-20}
processor=${CUDAOPS_PROCESSOR:-cudaops-process}
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

run_device() {
  local device=$1
  local output="$work_dir/$device.png"
  "$processor" --input "$image" --output "$output" --device "$device" >/dev/null
  for ((i=0; i<runs; i++)); do
    "$processor" --input "$image" --output "$output" --device "$device" |
      sed -n 's/.*"total_ms":\([0-9.]*\).*/\1/p'
  done | sort -n >"$work_dir/$device.times"
}

run_device cpu
run_device cuda

python3 - "$work_dir/cpu.times" "$work_dir/cuda.times" <<'PY'
import statistics, sys

def report(path):
    values = [float(line) for line in open(path, encoding="utf-8")]
    ordered = sorted(values)
    p95 = ordered[max(0, round(0.95 * len(ordered) + 0.5) - 1)]
    return statistics.median(values), p95

cpu = report(sys.argv[1])
cuda = report(sys.argv[2])
print(f"device  median_ms  p95_ms")
print(f"cpu     {cpu[0]:9.3f}  {cpu[1]:7.3f}")
print(f"cuda    {cuda[0]:9.3f}  {cuda[1]:7.3f}")
print(f"median speedup: {cpu[0] / cuda[0]:.2f}x")
PY

