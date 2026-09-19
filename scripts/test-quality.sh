#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
gremlins=$1
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

expect_failure() {
    if "$@" > "$fixture/rejected.log" 2>&1; then
        cat "$fixture/rejected.log" >&2
        echo "门禁错误地放行了失败样本: $*" >&2
        exit 1
    fi
}

# 格式门禁必须同时验证工具退出状态和未格式化文件列表。
mkdir "$fixture/bin"
cat > "$fixture/bin/gofmt" <<'SH'
#!/bin/sh
printf '%s' "$FORMAT_OUTPUT"
exit "$FORMAT_EXIT"
SH
chmod +x "$fixture/bin/gofmt"
expect_failure env PATH="$fixture/bin:$PATH" FORMAT_OUTPUT= FORMAT_EXIT=2 make -C "$script_dir/.." format-check
expect_failure env PATH="$fixture/bin:$PATH" FORMAT_OUTPUT=unformatted.go FORMAT_EXIT=0 make -C "$script_dir/.." format-check
env PATH="$fixture/bin:$PATH" FORMAT_OUTPUT= FORMAT_EXIT=0 make -C "$script_dir/.." format-check

printf 'mode: atomic\na.go:1.1,2.1 2030 1\na.go:3.1,4.1 1 0\n' | expect_failure awk -f "$script_dir/check-coverage.awk"
printf 'mode: atomic\n' | expect_failure awk -f "$script_dir/check-coverage.awk"
printf '损坏的报告\n' | expect_failure awk -f "$script_dir/check-coverage.awk"
printf 'mode: atomic\na.go:1.1,2.1 1 invalid\n' | expect_failure awk -f "$script_dir/check-coverage.awk"
printf '' | expect_failure awk -f "$script_dir/check-coverage.awk"
printf 'mode: atomic\na.go:1.1,2.1 2031 1\n' | awk -f "$script_dir/check-coverage.awk"

cat > "$fixture/perfect.json" <<'JSON'
{"test_efficacy":100,"mutations_coverage":100,"mutants_total":2,"mutants_killed":1,"mutants_lived":0,"mutants_not_covered":0,"mutants_not_viable":1,"files":[{"mutations":[{"status":"KILLED"},{"status":"NOT VIABLE"}]}]}
JSON
jq -er -f "$script_dir/check-mutations.jq" "$fixture/perfect.json"
for mutation in \
    '.test_efficacy = 99.999' \
    '.mutations_coverage = 99.999' \
    '.mutants_lived = 1' \
    '.mutants_not_covered = 1' \
    '.files[0].mutations[0].status = "TIMED OUT"' \
    '.files[0].mutations[0].status = "SKIPPED"' \
    '.files[0].mutations[0].status = "RUNNABLE"' \
    '.files = []' \
    'del(.mutants_killed)' \
    '.mutants_killed = 0'; do
    jq "$mutation" "$fixture/perfect.json" | expect_failure jq -er -f "$script_dir/check-mutations.jq"
done
printf '损坏的 JSON' | expect_failure jq -er -f "$script_dir/check-mutations.jq"

# 真实工具集成测试：同时验证有效率、变异覆盖率和 100% 边界。
mkdir "$fixture/module"
cat > "$fixture/module/go.mod" <<'GO'
module qualityfixture

go 1.24.0
GO
cat > "$fixture/module/range.go" <<'GO'
package qualityfixture

func Inside(value int) bool { return value > 0 && value < 10 }
GO
cat > "$fixture/module/range_test.go" <<'GO'
package qualityfixture
import "testing"
func TestInside(t *testing.T) {
 if !Inside(5) { t.Fatal("区间内应为真") }
}
GO
expect_failure bash "$script_dir/mutate.sh" "$gremlins" "$fixture/module"
jq -e '.test_efficacy < 100 and .mutants_lived > 0' "$fixture/module/gremlins-report.json" > /dev/null
cat > "$fixture/module/range_test.go" <<'GO'
package qualityfixture
import "testing"
func TestInside(t *testing.T) {
 for _, tc := range []struct{ value int; want bool }{{0,false},{1,true},{9,true},{10,false}} {
  if got := Inside(tc.value); got != tc.want { t.Fatalf("%d: %v", tc.value, got) }
 }
}
GO
bash "$script_dir/mutate.sh" "$gremlins" "$fixture/module"
cat >> "$fixture/module/range.go" <<'GO'

func Outside(value int) bool { return value < 0 }
GO
expect_failure bash "$script_dir/mutate.sh" "$gremlins" "$fixture/module"
jq -e '.mutations_coverage < 100 and .mutants_not_covered > 0' "$fixture/module/gremlins-report.json" > /dev/null

# 构建失败不能被残留的达标报告掩盖。
printf '无法编译的 Go 文件\n' > "$fixture/module/range.go"
cp "$fixture/perfect.json" "$fixture/module/gremlins-report.json"
expect_failure bash "$script_dir/mutate.sh" "$gremlins" "$fixture/module"
test ! -f "$fixture/module/gremlins-report.json"
echo '门禁自测通过：不足 100%、空报告、异常结果和构建失败均被拒绝，100% 样本通过'
