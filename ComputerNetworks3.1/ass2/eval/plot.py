#!/usr/bin/env python3
"""
eval/plot.py — Assignment 2 Evaluation Plots

Reads eval/results.csv and per-frame sender CSV logs to produce:
  1. Efficiency vs Error Probability   (line chart, one line per protocol)
  2. Retransmissions vs Error Probability (bar chart)
  3. RTT Distribution per Protocol        (box plot, from per-frame logs)
  4. Throughput vs Window Size            (line chart at fixed error rate)

Usage:
    python3 eval/plot.py
"""

import os
import glob
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
import numpy as np

# ── Styling ───────────────────────────────────────────────────────────────────
plt.style.use("seaborn-v0_8-darkgrid")
COLORS = {
    "stop-wait":  "#E63946",
    "gbn_w4":     "#457B9D",
    "gbn_w8":     "#1D3557",
    "sr-nonak_w4":"#2A9D8F",
    "sr-nonak_w8":"#264653",
    "sr-nak_w4":  "#F4A261",
    "sr-nak_w8":  "#E76F51",
}
os.makedirs("eval/plots", exist_ok=True)

# ── Load results.csv ──────────────────────────────────────────────────────────
results_path = "eval/results.csv"
if not os.path.exists(results_path):
    print(f"ERROR: {results_path} not found. Run eval/run_eval.sh first.")
    exit(1)

df = pd.read_csv(results_path)
df.columns = df.columns.str.strip()

# Build a human-readable label
def make_label(row):
    mode = row["mode"]
    w = int(row["window"])
    if mode == "stop-wait":
        return "stop-wait"
    return f"{mode}_w{w}"

df["label"] = df.apply(make_label, axis=1)
df["loss_prob"] = df["loss_prob"].astype(float)
df["efficiency_pct"] = df["efficiency_pct"].astype(float)
df["retransmits"] = df["retransmits"].astype(int)
df["total_time_ms"] = df["total_time_ms"].astype(float)

# ── Plot 1: Efficiency vs Error Probability ───────────────────────────────────
fig, ax = plt.subplots(figsize=(9, 5))
for label, grp in df.groupby("label"):
    grp_sorted = grp.sort_values("loss_prob")
    ax.plot(
        grp_sorted["loss_prob"],
        grp_sorted["efficiency_pct"],
        marker="o", linewidth=2,
        label=label,
        color=COLORS.get(label, None),
    )
ax.set_xlabel("Error / Loss Probability", fontsize=12)
ax.set_ylabel("Efficiency (%)", fontsize=12)
ax.set_title("Transmission Efficiency vs Error Probability", fontsize=13, fontweight="bold")
ax.set_ylim(0, 110)
ax.yaxis.set_major_formatter(mticker.PercentFormatter())
ax.legend(fontsize=9, ncol=2)
plt.tight_layout()
plt.savefig("eval/plots/efficiency_vs_error.png", dpi=150)
print("Saved eval/plots/efficiency_vs_error.png")
plt.close()

# ── Plot 2: Retransmissions vs Error Probability ──────────────────────────────
labels_sorted = sorted(df["label"].unique())
x = np.arange(len(df["loss_prob"].unique()))
loss_vals = sorted(df["loss_prob"].unique())
width = 0.8 / len(labels_sorted)

fig, ax = plt.subplots(figsize=(11, 5))
for i, label in enumerate(labels_sorted):
    grp = df[df["label"] == label].set_index("loss_prob")["retransmits"].reindex(loss_vals, fill_value=0)
    offset = (i - len(labels_sorted) / 2 + 0.5) * width
    ax.bar(x + offset, grp.values, width=width*0.9,
           label=label, color=COLORS.get(label, None), alpha=0.85)

ax.set_xticks(x)
ax.set_xticklabels([f"{v:.1f}" for v in loss_vals])
ax.set_xlabel("Error / Loss Probability", fontsize=12)
ax.set_ylabel("Retransmissions", fontsize=12)
ax.set_title("Retransmissions vs Error Probability", fontsize=13, fontweight="bold")
ax.legend(fontsize=9, ncol=2)
plt.tight_layout()
plt.savefig("eval/plots/retransmissions_vs_error.png", dpi=150)
print("Saved eval/plots/retransmissions_vs_error.png")
plt.close()

# ── Plot 3: RTT Distribution per Protocol (from per-frame sender CSVs) ────────
rtt_data = {}
for csv_path in sorted(glob.glob("logs/*_tx_*.csv")):
    try:
        d = pd.read_csv(csv_path)
        if "rtt_ms" not in d.columns:
            continue
        # Extract protocol label from filename
        fname = os.path.basename(csv_path)
        # Expected: <mode>_w<N>_loss<X>_tx_<mode>_loss<X>.csv
        # Simpler: try to find matching label from results
        rtts = pd.to_numeric(d["rtt_ms"], errors="coerce").dropna()
        if len(rtts) == 0:
            continue
        # Use filename as key (simplified)
        key = fname.replace("_tx_", "_").replace(".csv", "")
        if key not in rtt_data:
            rtt_data[key] = []
        rtt_data[key].extend(rtts.tolist())
    except Exception:
        continue

if rtt_data:
    fig, ax = plt.subplots(figsize=(max(8, len(rtt_data) * 1.5), 5))
    bp_data  = list(rtt_data.values())
    bp_labels = [k[:30] for k in rtt_data.keys()]  # truncate long names
    ax.boxplot(bp_data, labels=bp_labels, patch_artist=True,
               boxprops=dict(facecolor="#AED6F1"),
               medianprops=dict(color="#E74C3C", linewidth=2))
    ax.set_xlabel("Run", fontsize=11)
    ax.set_ylabel("RTT (ms)", fontsize=11)
    ax.set_title("RTT Distribution per Protocol Run", fontsize=13, fontweight="bold")
    plt.xticks(rotation=30, ha="right", fontsize=7)
    plt.tight_layout()
    plt.savefig("eval/plots/rtt_distribution.png", dpi=150)
    print("Saved eval/plots/rtt_distribution.png")
    plt.close()
else:
    print("No per-frame sender CSVs found — skipping RTT box plot")

# ── Plot 4: Total Time vs Protocol (at loss=0.2) ──────────────────────────────
subset = df[df["loss_prob"] == 0.2].sort_values("label")
if not subset.empty:
    fig, ax = plt.subplots(figsize=(9, 4))
    colors_list = [COLORS.get(l, "#888") for l in subset["label"]]
    bars = ax.bar(subset["label"], subset["total_time_ms"] / 1000,
                  color=colors_list, alpha=0.85, edgecolor="white")
    ax.set_xlabel("Protocol", fontsize=12)
    ax.set_ylabel("Total Time (s)", fontsize=12)
    ax.set_title("Total Transmission Time at loss=0.2", fontsize=13, fontweight="bold")
    plt.xticks(rotation=20, ha="right")
    plt.tight_layout()
    plt.savefig("eval/plots/total_time_loss02.png", dpi=150)
    print("Saved eval/plots/total_time_loss02.png")
    plt.close()

print("\nAll plots written to eval/plots/")
