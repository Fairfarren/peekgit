NR == 1 {
    if ($0 !~ /^mode: (set|count|atomic)$/) {
        print "覆盖报告缺少有效的模式声明" > "/dev/stderr"
        invalid = 1
    }
    next
}
{
    if (NF != 3 || $1 !~ /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$/ || $2 !~ /^[0-9]+$/ || $3 !~ /^[0-9]+$/) {
        print "覆盖报告格式错误: " $0 > "/dev/stderr"
        invalid = 1
        next
    }
    total += $2
    if ($3 == 0 && $2 > 0) {
        print "未覆盖语句: " $1 > "/dev/stderr"
        missing += $2
    }
}
END {
    if (invalid || total == 0 || missing > 0) {
        printf "覆盖门禁失败: %d/%d 条语句，要求精确 100%%\n", total - missing, total > "/dev/stderr"
        exit 1
    }
    printf "覆盖门禁通过: %d/%d 条语句\n", total, total
}
