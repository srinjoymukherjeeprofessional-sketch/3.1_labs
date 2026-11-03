#!/usr/bin/env python3
"""
eval/plot.py — Protocol Comparison Plots (Non-Persistent, 1-Persistent, p-Persistent p=1/N, CSMA/CD)

Reads eval/results.csv and produces:
  1. Throughput (kbps) vs Contending Stations (N)
  2. Transmission Delay (ms) vs Contending Stations (N)
  3. Total Collisions vs Contending Stations (N)
  4. Channel Efficiency (%) vs Contending Stations (N)
  5. 2x2 Comprehensive Multi-Metric Comparison Grid

Usage:
  python3 eval/plot.py [results_csv] [output_dir]
"""

import sys
import os
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
import numpy as np

try:
    plt.style.use("seaborn-v0_8-darkgrid")
except Exception:
    plt.style.use("ggplot")

COLORS = {
    "CSMA/CD (BEB)":                 "#1D3557",  # Deep navy
    "p-Persistent CSMA (best p=1/N)": "#2A9D8F",  # Emerald teal
    "Non-Persistent CSMA":           "#457B9D",  # Slate blue
    "1-Persistent CSMA":             "#E63946",  # Coral red
}

MARKERS = {
    "CSMA/CD (BEB)":                 "o",
    "p-Persistent CSMA (best p=1/N)": "s",
    "Non-Persistent CSMA":           "D",
    "1-Persistent CSMA":             "X",
}

LINESTYLES = {
    "CSMA/CD (BEB)":                 "-",
    "p-Persistent CSMA (best p=1/N)": "--",
    "Non-Persistent CSMA":           ":",
    "1-Persistent CSMA":             "-",
}

def format_protocol_label(row):
    mode = str(row["mode"]).strip()
    if mode == "csma-cd":
        return "CSMA/CD (BEB)"
    elif mode == "non-persistent":
        return "Non-Persistent CSMA"
    elif mode == "1-persistent":
        return "1-Persistent CSMA"
    elif mode == "p-persistent":
        return "p-Persistent CSMA (best p=1/N)"
    return mode

def main():
    csv_path = sys.argv[1] if len(sys.argv) > 1 else "eval/results.csv"
    out_dir = sys.argv[2] if len(sys.argv) > 2 else "eval/plots"

    if not os.path.exists(csv_path):
        print(f"Error: {csv_path} does not exist. Run eval/run_eval.sh first.")
        sys.exit(1)

    os.makedirs(out_dir, exist_ok=True)

    df = pd.read_csv(csv_path)
    df.columns = df.columns.str.strip()
    df["label"] = df.apply(format_protocol_label, axis=1)

    df["num_stations"] = df["num_stations"].astype(int)
    df["throughput_kbps"] = df["throughput_kbps"].astype(float)
    df["efficiency_pct"] = df["efficiency_pct"].astype(float)
    df["total_collisions"] = df["total_collisions"].astype(int)

    # Use avg_delay_ms if present, otherwise approximate duration / total_attempts
    if "avg_delay_ms" in df.columns:
        df["avg_delay_ms"] = df["avg_delay_ms"].astype(float)
    else:
        df["avg_delay_ms"] = df["duration_ms"].astype(float) / df["total_attempts"].astype(float)

    df = df.sort_values(by=["label", "num_stations"])

    ordered_labels = [
        "CSMA/CD (BEB)",
        "p-Persistent CSMA (best p=1/N)",
        "Non-Persistent CSMA",
        "1-Persistent CSMA",
    ]
    labels = [lbl for lbl in ordered_labels if lbl in df["label"].unique()]

    # ── 1. Throughput vs Stations ─────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    for lbl in labels:
        sub = df[df["label"] == lbl]
        ax.plot(
            sub["num_stations"], sub["throughput_kbps"],
            label=lbl,
            color=COLORS.get(lbl, "#333"),
            marker=MARKERS.get(lbl, "o"),
            linestyle=LINESTYLES.get(lbl, "-"),
            linewidth=2.2,
            markersize=7,
        )
    ax.set_title("Throughput vs Number of Contending Stations (N)", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Number of Competing Stations (N)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Throughput (kbps)", fontsize=11, fontweight="bold")
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="best")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "throughput_vs_stations.png"))
    plt.close(fig)

    # ── 2. Transmission Delay vs Stations ─────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    for lbl in labels:
        sub = df[df["label"] == lbl]
        ax.plot(
            sub["num_stations"], sub["avg_delay_ms"],
            label=lbl,
            color=COLORS.get(lbl, "#333"),
            marker=MARKERS.get(lbl, "s"),
            linestyle=LINESTYLES.get(lbl, "-"),
            linewidth=2.2,
            markersize=7,
        )
    ax.set_title("Average Transmission Delay vs Number of Contending Stations (N)", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Number of Competing Stations (N)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Average Transmission Delay (ms)", fontsize=11, fontweight="bold")
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="best")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "delay_vs_stations.png"))
    plt.close(fig)

    # ── 3. Total Collisions vs Stations ───────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    for lbl in labels:
        sub = df[df["label"] == lbl]
        ax.plot(
            sub["num_stations"], sub["total_collisions"],
            label=lbl,
            color=COLORS.get(lbl, "#333"),
            marker=MARKERS.get(lbl, "D"),
            linestyle=LINESTYLES.get(lbl, "-"),
            linewidth=2.2,
            markersize=7,
        )
    ax.set_title("Total Collision Episodes vs Station Count (N)", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Number of Competing Stations (N)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Total Collision Events", fontsize=11, fontweight="bold")
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="best")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "collisions_vs_stations.png"))
    plt.close(fig)

    # ── 4. Channel Efficiency vs Stations ─────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    for lbl in labels:
        sub = df[df["label"] == lbl]
        ax.plot(
            sub["num_stations"], sub["efficiency_pct"],
            label=lbl,
            color=COLORS.get(lbl, "#333"),
            marker=MARKERS.get(lbl, "^"),
            linestyle=LINESTYLES.get(lbl, "-"),
            linewidth=2.2,
            markersize=7,
        )
    ax.set_title("Channel Efficiency (%) vs Number of Contending Stations (N)", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Number of Competing Stations (N)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Channel Efficiency (%)", fontsize=11, fontweight="bold")
    ax.yaxis.set_major_formatter(mticker.PercentFormatter(xmax=100))
    ax.set_ylim(-2, 102)
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="best")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "efficiency_vs_stations.png"))
    plt.close(fig)

    # ── 5. Comprehensive 2x2 Grid ─────────────────────────────────────────────
    fig, axs = plt.subplots(2, 2, figsize=(14, 10), dpi=300)

    # (0, 0) Throughput
    for lbl in labels:
        sub = df[df["label"] == lbl]
        axs[0, 0].plot(
            sub["num_stations"], sub["throughput_kbps"],
            label=lbl, color=COLORS.get(lbl, "#333"), marker=MARKERS.get(lbl, "o"),
            linestyle=LINESTYLES.get(lbl, "-"), linewidth=2.2
        )
    axs[0, 0].set_title("(a) Throughput (kbps)", fontweight="bold")
    axs[0, 0].set_xlabel("Stations (N)")
    axs[0, 0].set_ylabel("Throughput (kbps)")
    axs[0, 0].grid(True, linestyle="--", alpha=0.6)

    # (0, 1) Efficiency
    for lbl in labels:
        sub = df[df["label"] == lbl]
        axs[0, 1].plot(
            sub["num_stations"], sub["efficiency_pct"],
            label=lbl, color=COLORS.get(lbl, "#333"), marker=MARKERS.get(lbl, "s"),
            linestyle=LINESTYLES.get(lbl, "-"), linewidth=2.2
        )
    axs[0, 1].set_title("(b) Channel Efficiency (%)", fontweight="bold")
    axs[0, 1].set_xlabel("Stations (N)")
    axs[0, 1].set_ylabel("Efficiency (%)")
    axs[0, 1].yaxis.set_major_formatter(mticker.PercentFormatter(xmax=100))
    axs[0, 1].grid(True, linestyle="--", alpha=0.6)

    # (1, 0) Collisions
    for lbl in labels:
        sub = df[df["label"] == lbl]
        axs[1, 0].plot(
            sub["num_stations"], sub["total_collisions"],
            label=lbl, color=COLORS.get(lbl, "#333"), marker=MARKERS.get(lbl, "D"),
            linestyle=LINESTYLES.get(lbl, "-"), linewidth=2.2
        )
    axs[1, 0].set_title("(c) Total Collision Events", fontweight="bold")
    axs[1, 0].set_xlabel("Stations (N)")
    axs[1, 0].set_ylabel("Collisions")
    axs[1, 0].grid(True, linestyle="--", alpha=0.6)

    # (1, 1) Transmission Delay
    for lbl in labels:
        sub = df[df["label"] == lbl]
        axs[1, 1].plot(
            sub["num_stations"], sub["avg_delay_ms"],
            label=lbl, color=COLORS.get(lbl, "#333"), marker=MARKERS.get(lbl, "^"),
            linestyle=LINESTYLES.get(lbl, "-"), linewidth=2.2
        )
    axs[1, 1].set_title("(d) Average Transmission Delay (ms)", fontweight="bold")
    axs[1, 1].set_xlabel("Stations (N)")
    axs[1, 1].set_ylabel("Delay (ms)")
    axs[1, 1].grid(True, linestyle="--", alpha=0.6)

    handles, plot_labels = axs[0, 0].get_legend_handles_labels()
    fig.legend(handles, plot_labels, loc="lower center", ncol=len(labels), bbox_to_anchor=(0.5, 0.01), fontsize=10, frameon=True)

    fig.suptitle("MAC Protocol Performance as Functions of Contending Stations (N)", fontsize=15, fontweight="bold", y=0.98)
    plt.tight_layout(rect=[0, 0.05, 1, 0.96])
    fig.savefig(os.path.join(out_dir, "comprehensive_comparison.png"))
    plt.close(fig)

    print(f"All evaluation plots successfully generated in: {out_dir}/")
    print(f"  - {os.path.join(out_dir, 'throughput_vs_stations.png')}")
    print(f"  - {os.path.join(out_dir, 'delay_vs_stations.png')}")
    print(f"  - {os.path.join(out_dir, 'collisions_vs_stations.png')}")
    print(f"  - {os.path.join(out_dir, 'efficiency_vs_stations.png')}")
    print(f"  - {os.path.join(out_dir, 'comprehensive_comparison.png')}")

if __name__ == "__main__":
    main()
