#!/usr/bin/env bash
# Go 要求 _test.go 与被测包同目录，而这个项目的测试文件都收在 test/。
# 这个脚本把它们临时放回仓库根目录，跑完 go vet / go test 再收回来，
# 中途失败或 Ctrl+C 也会收回。
#
# 用法：bash test/run.sh [go test 参数，例如 -run TestToolbar -v]

set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

moved=()
restore() {
	for name in ${moved[@]+"${moved[@]}"}; do
		if [ -f "$name" ]; then
			mv -f "$name" test/
		fi
	done
}
trap restore EXIT INT TERM

for path in test/*_test.go; do
	[ -e "$path" ] || continue
	name="$(basename "$path")"
	if [ -e "$name" ]; then
		printf '%s 已存在于仓库根目录，先收回到 test/ 再重试\n' "$name" >&2
		exit 1
	fi
	mv "$path" "$name"
	moved+=("$name")
done

if [ "${#moved[@]}" -eq 0 ]; then
	printf 'test/ 下没有测试文件\n' >&2
	exit 1
fi

printf '已放出 %d 个测试文件，开始检查\n' "${#moved[@]}"
go vet .
go test -count=1 "$@" .
