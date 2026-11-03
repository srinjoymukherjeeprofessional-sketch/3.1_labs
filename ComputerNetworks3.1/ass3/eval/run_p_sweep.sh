#!/usr/bin/env bash
set -e

# CSMA p-Persistence Parameter Sweep (Fixed N)
# Evaluates Number of Collisions, Transmission Delay, and Throughput as functions of p.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_SIM="$ROOT_DIR/bin/sim"
P_CSV="$SCRIPT_DIR/p_sweep_results.csv"

FIXED_N=${1:-5}
FRAMES=${2:-25}
TP="5ms"
TFR="10ms"

echo "================================================================="
echo "   p-Persistent CSMA: Evaluation as a Function of p              "
echo "================================================================="
echo "  Fixed Stations (N)   : $FIXED_N"
echo "  Frames per Station   : $FRAMES"
echo "  Propagation Delay    : $TP"
echo "  Frame Duration       : $TFR"
echo "  Output CSV           : $P_CSV"
echo "================================================================="

rm -f "$P_CSV"

P_VALUES=(0.02 0.05 0.10 0.15 0.20 0.25 0.30 0.40 0.50 0.60 0.70 0.80 0.90 1.00)

for P in "${P_VALUES[@]}"; do
    echo "  -> Running p-Persistent CSMA with p=$P (N=$FIXED_N)..."
    "$BIN_SIM" -mode p-persistent -p "$P" -stations "$FIXED_N" -frames "$FRAMES" -tp "$TP" -tfr "$TFR" -quiet -csv "$P_CSV"
done

echo ""
echo "p-Persistence sweep complete! Results written to $P_CSV"
echo "Generating plots for Collisions, Delay, and Throughput as functions of p..."
python3 "$SCRIPT_DIR/plot_p_sweep.py" "$P_CSV" "$SCRIPT_DIR/plots" "$FIXED_N"
echo "================================================================="
