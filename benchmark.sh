#!/usr/bin/env bash
set -euo pipefail

read -r -p "Enter TREEBALL_BIN path [./treeball]: " input
TREEBALL_BIN="${input:-./treeball}"

read -r -p "Enter RESULTS file path [./treeball_benchmark.txt]: " input
RESULTS="${input:-./treeball_benchmark.txt}"

read -r -p "Enter TMP_LOG file path [./treeball_benchmark_tmp.txt]: " input
TMP_LOG="${input:-./treeball_benchmark_tmp.txt}"

echo ""
echo "!! ENSURE ENOUGH INODES + DISK SPACE FOR THE FOLLOWING DIRS:"
echo "!! BENCH_DIR NEEDS AT LEAST AS MANY INODES AS SIZES BENCHMARKED."
echo "!! TMP_DIR NEEDS AT LEAST MULTIPLE GIGABYTES FOR EXTERNAL SORTING."
echo ""

read -r -p "Enter BENCH_DIR path [./treeball_benchmark]: " input
BENCH_DIR="${input:-./treeball_benchmark}"

read -r -p "Enter TMP_DIR path [./treeball_benchmark_tmp]: " input
TMP_DIR="${input:-./treeball_benchmark_tmp}"

read -r -p "Enter SIZES (space-separated) [5000 10000 50000 100000 500000 1000000 5000000]: " input
SIZES=(${input:-5000 10000 50000 100000 500000 1000000 5000000})

cleanup() {
    rm -f "$TMP_LOG"
    rm -rf "$BENCH_DIR"/trbb_* "$TMP_DIR"/trbb_*
}
trap cleanup EXIT

mkdir -p "$BENCH_DIR"
mkdir -p "$TMP_DIR"
> "$RESULTS"

log() {
    echo "[$(date +'%H:%M:%S')] $*" >&2
}

drop_caches() {
    sync
    if [[ $EUID -eq 0 ]]; then
        echo 3 > /proc/sys/vm/drop_caches
    else
        if sudo -n true 2>/dev/null; then
            sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'
        else
            echo ""
            echo "!! NOW REQUESTING SUDO TO DROP THE KERNEL CACHES:" >&2
            sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'
            echo ""
        fi
    fi
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
    local root="$BENCH_DIR/trbb_$count"
    local tar1="$TMP_DIR/trbb_${count}_a.tar.gz"
    local tar2="$TMP_DIR/trbb_${count}_b.tar.gz"
    local diff="$TMP_DIR/trbb_diff_${count}.tar.gz"

    log "Generating directory with $count files..."
    rm -rf "$root"
    create_dummy_tree "$root" "$count"

    echo -e "\n--- $count FILES ---" >> "$RESULTS"
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

    # 4. DIFF - TAR/TAR (ignore exit 1)
    drop_caches
    set +e
    /usr/bin/time -f "DIFF TAR/TAR wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" diff "$tar1" "$tar2" "$diff" --tmpdir="$TMP_DIR" &> /dev/null
    set -e
    extract_and_log_metrics "DIFF TAR/TAR"

    # 5. DIFF - TAR/FOLDER (ignore exit 1)
    drop_caches
    set +e
    /usr/bin/time -f "DIFF TAR/FOLDER wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" diff "$tar1" "$root" "$diff" --tmpdir="$TMP_DIR" &> /dev/null
    set -e
    extract_and_log_metrics "DIFF TAR/FOLDER"

    # 6. DIFF - FOLDER/FOLDER (ignore exit 1)
    drop_caches
    set +e
    /usr/bin/time -f "DIFF FOLDER/FOLDER wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" diff "$root" "$root" "$diff" --tmpdir="$TMP_DIR" &> /dev/null
    set -e
    extract_and_log_metrics "DIFF FOLDER/FOLDER"

    # 7. LIST
    drop_caches
    /usr/bin/time -f "LIST wall: %e sec, user: %U, sys: %S, RAM: %M KB" -a -o "$TMP_LOG" \
        "$TREEBALL_BIN" list "$tar2" --tmpdir="$TMP_DIR" > /dev/null
    extract_and_log_metrics "LIST"

    avg_len=$(find "$root" -type f | awk '{ total += length($0); count++ } END { if (count > 0) print int(total/count); else print 0 }')
    echo "Average path length: ${avg_len} characters" >> "$RESULTS"

    max_depth=$(find "$root" -type f -printf '%d\n' | sort -n | tail -1)
    echo "Maximum path depth: ${max_depth} levels" >> "$RESULTS"

    cat "$TMP_LOG" >> "$RESULTS"
    rm -f "$tar1" "$tar2" "$diff" "$TMP_LOG"
    rm -rf "$root"
}

command -v go >/dev/null 2>&1 || {
    echo "Go compiler not found in PATH" >&2
    exit 1
}

echo ""
echo "===========================" | tee -a "$RESULTS"
date | tee -a "$RESULTS"
echo "===========================" | tee -a "$RESULTS"

echo "" | tee -a "$RESULTS"
echo "TREEBALL_BIN=$TREEBALL_BIN" | tee -a "$RESULTS"
echo "RESULTS=$RESULTS" | tee -a "$RESULTS"
echo "TMP_LOG=$TMP_LOG" | tee -a "$RESULTS"
echo "BENCH_DIR=$BENCH_DIR" | tee -a "$RESULTS"
echo "TMP_DIR=$TMP_DIR" | tee -a "$RESULTS"
echo "SIZES=(${SIZES[*]})" | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

echo "CPU cores: $(nproc)" | tee -a "$RESULTS"
echo "Filesystem type: $(df -T "$BENCH_DIR" | awk 'NR==2 {print $2}')" | tee -a "$RESULTS"
check_treeball

echo ""
log "Starting benchmarks..."

for size in "${SIZES[@]}"; do
    run_benchmarks "$size"
done

echo "" | tee -a "$RESULTS"
echo "BENCHMARK COMPLETE" | tee -a "$RESULTS"
echo "" | tee -a "$RESULTS"

cat "$RESULTS"
