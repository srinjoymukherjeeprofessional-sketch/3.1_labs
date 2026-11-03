#!/usr/bin/env bash
# eval/run_eval.sh
#
# Automated evaluation harness for Assignment 2.
#
# Runs all combinations of:
#   - Protocol: stop-wait, gbn (N=4,8), sr-nonak (N=4,8), sr-nak (N=4,8)
#   - Error probability: 0.0, 0.1, 0.2, 0.3, 0.5
#
# For each combination the script:
#   1. Writes sender + receiver config files
#   2. Starts the receiver in the background
#   3. Starts the sender (waits for it to finish)
#   4. Compares output.txt with input.txt (integrity check)
#   5. Collects the summary appended to eval/results.csv
#
# Usage:
#   cd ass2
#   bash eval/run_eval.sh
#
# Output:
#   eval/results.csv   — one row per run (mode, window, loss, efficiency, …)
#   logs/              — per-frame CSV logs for each run

set -euo pipefail

SENDER_BIN="./sender/sender"
RECEIVER_BIN="./receiver/receiver"
INPUT="input.txt"
PORT=9001

# Build binaries first
echo "==> Building binaries..."
go build -o "$SENDER_BIN" ./sender
go build -o "$RECEIVER_BIN" ./receiver

mkdir -p eval logs

# Initialise results CSV with header (truncate if exists)
RESULTS="eval/results.csv"
echo "mode,window,fcs,loss_prob,corrupt_prob,total_frames,retransmits,efficiency_pct,total_time_ms" > "$RESULTS"

# ── Test matrix ───────────────────────────────────────────────────────────────
MODES=("stop-wait" "gbn" "gbn" "sr-nonak" "sr-nonak" "sr-nak" "sr-nak")
WINDOWS=(1         4     8     4           8           4         8)
NAK_FLAGS=("false" "false" "false" "false" "false" "true" "true")

ERROR_PROBS=(0.0 0.1 0.2 0.3 0.5)
FCS="crc32"           # change to "checksum" to test the other algo
DELAY_MS=50

run_one() {
    local mode=$1
    local window=$2
    local nak=$3
    local loss=$4

    # Map "sr-nonak" / "sr-nak" → actual mode "sr"
    local go_mode="$mode"
    if [[ "$mode" == "sr-nonak" || "$mode" == "sr-nak" ]]; then
        go_mode="sr"
    fi

    local tag="${mode}_w${window}_loss${loss}"
    local rx_cfg="eval/rx_${tag}.cfg"
    local tx_cfg="eval/tx_${tag}.cfg"
    local out_file="eval/output_${tag}.txt"
    local log_pfx="logs/${tag}"

    echo ""
    echo "── Running: mode=${mode} window=${window} loss=${loss} ──"

    # Write receiver config
    cat > "$rx_cfg" <<EOF
listen=:${PORT}
mode=${go_mode}
fcs=${FCS}
window=${window}
nak=${nak}
ack_loss=${loss}
ack_delay=0.02
ack_delay_dur_ms=${DELAY_MS}
ack_corrupt=${loss}
output=${out_file}
log_prefix=${log_pfx}_rx
EOF

    # Write sender config
    cat > "$tx_cfg" <<EOF
remote=localhost:${PORT}
mode=${go_mode}
fcs=${FCS}
window=${window}
nak=${nak}
loss=${loss}
delay=0.02
delay_dur_ms=${DELAY_MS}
corrupt=${loss}
input=${INPUT}
log_prefix=${log_pfx}_tx
EOF

    # Start receiver in background
    "$RECEIVER_BIN" "$rx_cfg" &
    RX_PID=$!
    sleep 0.3   # give receiver time to bind

    # Run sender (blocks until done)
    "$SENDER_BIN" "$tx_cfg"

    # Wait for receiver to finish
    wait $RX_PID 2>/dev/null || true

    # Integrity check
    if diff -q "$INPUT" "$out_file" > /dev/null 2>&1; then
        echo "  ✓ Output matches input"
    else
        echo "  ✗ Output MISMATCH — check ${out_file}"
    fi
}

# ── Main loop ─────────────────────────────────────────────────────────────────
for i in "${!MODES[@]}"; do
    mode="${MODES[$i]}"
    window="${WINDOWS[$i]}"
    nak="${NAK_FLAGS[$i]}"
    for loss in "${ERROR_PROBS[@]}"; do
        run_one "$mode" "$window" "$nak" "$loss"
    done
done

echo ""
echo "==> Evaluation complete. Results in ${RESULTS}"
echo "==> Run: python3 eval/plot.py"
