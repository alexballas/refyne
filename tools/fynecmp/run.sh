#!/usr/bin/env bash
# Benchmark refyne against upstream fyne with identical benchmark code.
#
# The templates are written against fyne.io/fyne/v2 and copied into a fresh
# checkout of each framework with the import path rewritten, so both run the
# same scenarios: a public-API set through the software painter (cmpbench) and
# a GL frame set inside internal/driver/glfw, which needs a display.
#
# Usage: tools/fynecmp/run.sh [count] [bench-regex]
#
#   WORK=dir          where checkouts and results go (default /tmp/fynecmp)
#   UPSTREAM_REF=ref  fyne-io/fyne branch or tag to compare with (default develop)
#   REFYNE_REF=ref    refyne git ref to measure; unset measures the working tree
#   BENCHTIME=t       passed to -benchtime
#
# Results land in $WORK/results, with a benchstat comparison in compare.txt.
# Shots: tools/fynecmp/run.sh shot writes a GL capture of a mixed UI from each
# framework to $WORK/results/<name>.png, for diffing rendering changes.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../.." && pwd)
WORK=${WORK:-/tmp/fynecmp}
UPSTREAM_REF=${UPSTREAM_REF:-develop}
REFYNE_MOD=github.com/alexballas/refyne/v2
# Both trees are standalone modules; a go.work around them would swap one in.
export GOWORK=off
mkdir -p "$WORK/results"

fetch_trees() {
	if [ ! -d "$WORK/fyne/.git" ]; then
		git clone --quiet --depth 1 --branch "$UPSTREAM_REF" https://github.com/fyne-io/fyne.git "$WORK/fyne"
	else
		# An earlier run may have checked out another ref, or an older develop.
		git -C "$WORK/fyne" fetch --quiet --depth 1 origin "$UPSTREAM_REF"
		git -C "$WORK/fyne" checkout --quiet --force FETCH_HEAD
	fi
	rm -rf "$WORK/refyne" && mkdir -p "$WORK/refyne"
	if [ -n "${REFYNE_REF:-}" ]; then
		git -C "$REPO" archive "$REFYNE_REF" | tar -x -C "$WORK/refyne"
	else
		tar -C "$REPO" --exclude=.git -cf - . | tar -x -C "$WORK/refyne"
	fi
}

# install copies a template into a tree with fyne's import path rewritten.
install() { # template tree module dest
	sed "s#fyne.io/fyne/v2#$3#g" "$HERE/templates/$1.go.tmpl" >"$WORK/$2/$4"
}

if [ "${1:-}" = shot ]; then
	fetch_trees
	for t in fyne refyne; do
		mod=fyne.io/fyne/v2
		[ $t = refyne ] && mod=$REFYNE_MOD
		install glshot_test $t $mod internal/driver/glfw/glshot_test.go
		(cd "$WORK/$t" && GLSHOT_OUT="$WORK/results/$t.png" go test ./internal/driver/glfw -run TestGLShot -count=1)
	done
	exit 0
fi

COUNT=${1:-6}
FILTER=${2:-.}
fetch_trees
for t in fyne refyne; do
	mod=fyne.io/fyne/v2
	[ $t = refyne ] && mod=$REFYNE_MOD
	mkdir -p "$WORK/$t/cmpbench"
	install cmpbench_test $t $mod cmpbench/cmpbench_test.go
	install glcmp_bench_test $t $mod internal/driver/glfw/glcmp_bench_test.go
	(
		cd "$WORK/$t"
		go test ./cmpbench -run '^$' -bench "$FILTER" -benchmem -count "$COUNT" ${BENCHTIME:+-benchtime $BENCHTIME} -timeout 60m
		go test ./internal/driver/glfw -run '^$' -bench "BenchmarkGL_.*$FILTER" -benchmem -count "$COUNT" ${BENCHTIME:+-benchtime $BENCHTIME} -timeout 60m
	) >"$WORK/results/$t.txt"
	# the same package names let benchstat line the two runs up
	sed -i "s#$REFYNE_MOD#fyne.io/fyne/v2#" "$WORK/results/$t.txt"
done

if ! command -v benchstat >/dev/null; then
	echo "benchstat not found: go install golang.org/x/perf/cmd/benchstat@latest" >&2
	exit 1
fi
benchstat fyne="$WORK/results/fyne.txt" refyne="$WORK/results/refyne.txt" | tee "$WORK/results/compare.txt"
