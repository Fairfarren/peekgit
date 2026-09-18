#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
gremlins=$1
cd -- "$2"
rm -f gremlins-report.json
status=0
"$gremlins" unleash --config "$script_dir/../.gremlins.yaml" -o gremlins-report.json || status=$?

# v0.6.0 把等于阈值也判为失败；仅在原始报告确实达到 100% 时兼容 10/11。
# https://github.com/go-gremlins/gremlins/blob/v0.6.0/internal/report/report.go
case "$status" in
    0|10|11) ;;
    *) echo "变异测试运行失败，退出码: $status" >&2; exit "$status" ;;
esac
jq -er -f "$script_dir/check-mutations.jq" gremlins-report.json
