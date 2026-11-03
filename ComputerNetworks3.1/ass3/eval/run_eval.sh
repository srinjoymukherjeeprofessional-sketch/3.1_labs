#!/usr/bin/env bash
set -e

# CSMA and CSMA/CD Automated Benchmark Suite
# Compares 1-Persistent, Non-Persistent, p-Persistent (at best p = 1/N), and CSMA/CD
# as functions of the number of contending stations N.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_SIM="$ROOT_DIR/bin/sim"
RESULTS_CSV="$SCRIPT_DIR/results.csv"

echo "================================================================="
echo "   IEEE 802.3 CSMA / CSMA-CD Protocol Evaluation Suite           "
echo "   (Non-Persistent, 1-Persistent, p-Persistent p=1/N, CSMA/CD)   "
echo "================================================================="

# Compile latest binaries
mkdir -p "$ROOT_DIR/bin"
echo "Compiling simulation binaries..."
go build -o "$ROOT_DIR/bin/sim" "$ROOT_DIR/cmd/sim"
go build -o "$ROOT_DIR/bin/server" "$ROOT_DIR/cmd/server"
go build -o "$ROOT_DIR/bin/station" "$ROOT_DIR/cmd/station"
echo "Build complete."

# Prepare clean results file
rm -f "$RESULTS_CSV"

STATION_COUNTS=(2 4 6 8 12 16)
FRAMES=25
TP="5ms"
TFR="10ms"

echo ""
echo "Simulation Settings:"
echo "  - Stations Sweep     : ${STATION_COUNTS[*]}"
echo "  - Frames per Station : $FRAMES"
echo "  - Propagation Delay  : $TP"
echo "  - Frame Duration     : $TFR"
echo "  - Output CSV         : $RESULTS_CSV"
echo "================================================================="

# 1. Benchmark CSMA/CD
echo ""
echo "─── [1/4] Benchmarking CSMA/CD with Binary Exponential Backoff ───"
for N in "${STATION_COUNTS[@]}"; do
    echo "  -> Running CSMA/CD with N=$N stations..."
    "$BIN_SIM" -mode csma-cd -stations "$N" -frames "$FRAMES" -tp "$TP" -tfr "$TFR" -quiet -csv "$RESULTS_CSV"
done

# 2. Benchmark Non-Persistent CSMA
echo ""
echo "─── [2/4] Benchmarking Non-Persistent CSMA ───────────────────────"
for N in "${STATION_COUNTS[@]}"; do
    echo "  -> Running Non-Persistent CSMA with N=$N stations..."
    "$BIN_SIM" -mode non-persistent -stations "$N" -frames "$FRAMES" -tp "$TP" -tfr "$TFR" -quiet -csv "$RESULTS_CSV"
done

# 3. Benchmark 1-Persistent CSMA
echo ""
echo "─── [3/4] Benchmarking 1-Persistent CSMA ─────────────────────────"
for N in "${STATION_COUNTS[@]}"; do
    echo "  -> Running 1-Persistent CSMA with N=$N stations..."
    "$BIN_SIM" -mode 1-persistent -stations "$N" -frames "$FRAMES" -tp "$TP" -tfr "$TFR" -quiet -csv "$RESULTS_CSV"
done

# 4. Benchmark p-Persistent CSMA at optimal p = 1/N
echo ""
echo "─── [4/4] Benchmarking p-Persistent CSMA at Optimal p = 1/N ──────"
for N in "${STATION_COUNTS[@]}"; do
    P=$(awk -v n="$N" 'BEGIN { printf "%.4f", 1.0/n }')
    echo "  -> Running p-Persistent (optimal p = 1/$N = $P) with N=$N stations..."
    "$BIN_SIM" -mode p-persistent -p "$P" -stations "$N" -frames "$FRAMES" -tp "$TP" -tfr "$TFR" -quiet -csv "$RESULTS_CSV"
done

echo ""
echo "================================================================="
echo "All benchmarks completed successfully! Summary written to:"
echo "  $RESULTS_CSV"
echo "Generating graphical plots..."
python3 "$SCRIPT_DIR/plot.py" "$RESULTS_CSV" "$SCRIPT_DIR/plots"
echo "================================================================="
