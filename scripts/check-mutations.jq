# 使用逐条结果核对汇总，防止四舍五入、超时或跳过被当作通过。
[.files[].mutations[].status] as $statuses
| if .test_efficacy == 100
    and .mutations_coverage == 100
    and .mutants_killed > 0
    and .mutants_lived == 0
    and .mutants_not_covered == 0
    and (.mutants_not_viable | type) == "number"
    and ($statuses | map(select(. == "KILLED")) | length) == .mutants_killed
    and ($statuses | map(select(. == "NOT VIABLE")) | length) == .mutants_not_viable
    and ($statuses | length) == (.mutants_killed + .mutants_not_viable)
    and .mutants_total == ($statuses | length)
  then "变异门禁通过: 有效率 100%，覆盖率 100%，杀死 \(.mutants_killed) 个变异"
  else error("变异门禁失败: 有效率 \(.test_efficacy)%，覆盖率 \(.mutations_coverage)%，存活 \(.mutants_lived)，未覆盖 \(.mutants_not_covered)；要求双 100%，且没有超时或跳过")
  end
