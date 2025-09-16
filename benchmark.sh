#!/usr/bin/env bash
set -euo pipefail

TREEBALL_BIN="./treeball"
BASE_DIR="/mnt/treeball_xfs"
RESULTS="./treeball_bench.txt"
SIZES=(5000 10000 50000 100000 500000 1000000 5000000)
TMP_LOG="./treeball_bench_tmp.txt"

mkdir -p "$BASE_DIR"
> "$RESULTS"

log() {
    echo "[$(date +'%H:%M:%S')] $*" >&2
}

drop_caches() {
    sync
    echo 3 > /proc/sys/vm/drop_caches
}

check_treeball() {
    if ! "$TREEBALL_BIN" --version >> "$RESULTS"; then
        echo "Error: treeball binary not working at $TREEBALL_BIN" >&2
        exit 1
    fi
}

extract_and_log_metrics() {
    local label="$1"

    local U S W
    U=$(awk -v l="$label" '$0 ~ l" " {for(i=1;i<=NF;i++) if($i=="user:") print $(i+1)}' "$TMP_LOG")
    S=$(awk -v l="$label" '$0 ~ l" " {for(i=1;i<=NF;i++) if($i=="sys:") print $(i+1)}' "$TMP_LOG")
    W=$(awk -v l="$label" '$0 ~ l" " {for(i=1;i<=NF;i++) if($i=="wall:") print $(i+1)}' "$TMP_LOG")

    : "${U:=0}"
    : "${S:=0}"
    : "${W:=1}"

    if [[ "$W" != "1" || "$U" != "0" || "$S" != "0" ]]; then
        CPU=$(awk -v u="$U" -v s="$S" -v w="$W" 'BEGIN {cpu=(u+s)/w*100; printf "%.1f", cpu}')
        echo "$label CPU Utilization: $CPU%" >> "$RESULTS"
    fi
}

create_dummy_tree() {
    go run ./tools/create_bench_tree.go "$1" "$2"
}

run_benchmarks() {
    local count=$1
    local root="$BASE_DIR/root_$count"
    local tar1="tree_${count}_a.tar.gz"
    local tar2="tree_${count}_b.tar.gz"
    local diff="diff_${count}.tar.gz"

    log "Generating directory with $count files..."
    rm -rf "$root"
    create_dummy_tree "$root" "$count"

    echo -e "\n=== $count FILES ===" >> "$RESULTS"
    log "Benching with $count files..."

    # 1. CREATE
    drop_caches
    /usr/bin/time -f "CREATE wall: %e sec, user: %U, sys: %S, RAM: %M KB" -o "$TMP_LOG" \
        "$TREEBALL_BIN" create "$root" "$tar1" > /dev/null
    ls -lh "$tar1" | awk '{print "CREATE size: " $5}' >> "$TMP_LOG"
    extract_and_log_metrics "CREATE"

    # 2. Modify (add files)
    touch "$root/new_file_1" "$root/new_file_2"

    # 3. CREATE2
    drop_caches
    /usr/bin/time -f "CREATE2 wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" create "$root" "$tar2" > /dev/null
    ls -lh "$tar2" | awk '{print "CREATE2 size: " $5}' >> "$TMP_LOG"
    extract_and_log_metrics "CREATE2"

    # 4. DIFF (ignore exit 1)
    drop_caches
    set +e
    /usr/bin/time -f "DIFF wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" diff "$tar1" "$tar2" "$diff" --tmpdir="$BASE_DIR" &> /dev/null
    set -e
    ls -lh "$diff" | awk '{print "DIFF size: " $5}' >> "$TMP_LOG"
    extract_and_log_metrics "DIFF"

    # 5. LIST
    drop_caches
    /usr/bin/time -f "LIST wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" list "$tar2" --tmpdir="$BASE_DIR" > /dev/null
    extract_and_log_metrics "LIST"

    avg_len=$(find "$root" -type f | awk '{ total += length($0); count++ } END { if (count > 0) print int(total/count); else print 0 }')
    echo "Average path length: ${avg_len} characters" >> "$RESULTS"

    max_depth=$(find "$root" -type f -printf '%d\n' | sort -n | tail -1)
    echo "Maximum path depth: ${max_depth} levels" >> "$RESULTS"

    cat "$TMP_LOG" >> "$RESULTS"
    rm -f "$tar1" "$tar2" "$diff" "$TMP_LOG"
    rm -rf "$root"
}

if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root (to drop kernel caches)." >&2
  exit 1
fi

echo "CPU cores: $(nproc)" >> "$RESULTS"
echo "Filesystem type: $(df -T "$BASE_DIR" | awk 'NR==2 {print $2}')" >> "$RESULTS"

check_treeball

log "Starting benchmarks..."
for size in "${SIZES[@]}"; do
    run_benchmarks "$size"
done

log "Done. Results saved to $RESULTS."
cat "$RESULTS"
