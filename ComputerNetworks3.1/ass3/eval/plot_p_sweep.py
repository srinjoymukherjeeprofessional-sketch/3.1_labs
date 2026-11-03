#!/usr/bin/env python3
"""
eval/plot_p_sweep.py — Plot performance metrics as functions of p (Fixed N)

Reads eval/p_sweep_results.csv and produces:
  1. Number of Collisions vs p
  2. Transmission Delay (ms) vs p
  3. Throughput (kbps) vs p
  4. Combined 3-panel plot: (a) Collisions vs p, (b) Delay vs p, (c) Throughput vs p

Usage:
  python3 eval/plot_p_sweep.py [csv_path] [output_dir] [fixed_N]
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

def main():
    csv_path = sys.argv[1] if len(sys.argv) > 1 else "eval/p_sweep_results.csv"
    out_dir = sys.argv[2] if len(sys.argv) > 2 else "eval/plots"
    fixed_n = sys.argv[3] if len(sys.argv) > 3 else "5"

    if not os.path.exists(csv_path):
        print(f"Error: {csv_path} does not exist.")
        sys.exit(1)

    os.makedirs(out_dir, exist_ok=True)

    df = pd.read_csv(csv_path)
    df.columns = df.columns.str.strip()

    # Filter to p-persistent rows and sort by p_val
    df = df[df["mode"].str.contains("p-persistent", case=False, na=False)].copy()
    df["p_val"] = df["p_val"].astype(float)
    df["total_collisions"] = df["total_collisions"].astype(int)
    df["throughput_kbps"] = df["throughput_kbps"].astype(float)
    
    # If avg_delay_ms column exists, use it; otherwise compute from duration / total_attempts
    if "avg_delay_ms" in df.columns:
        df["avg_delay_ms"] = df["avg_delay_ms"].astype(float)
    else:
        df["avg_delay_ms"] = (df["duration_ms"].astype(float)) / (df["total_attempts"].astype(float))

    df = df.sort_values(by="p_val")

    color_primary = "#2A9D8F"  # Teal
    color_danger  = "#E63946"  # Red
    color_info    = "#1D3557"  # Navy

    # ── 1. Collisions vs p ────────────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    ax.plot(
        df["p_val"], df["total_collisions"],
        marker="o", color=color_danger, linewidth=2.4, markersize=7,
        label=f"p-Persistent CSMA (N={fixed_n})"
    )
    ax.set_title(f"p-Persistent CSMA: Total Collisions vs p (Fixed N={fixed_n})", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Transmission Probability (p)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Total Collision Events", fontsize=11, fontweight="bold")
    ax.set_xlim(0.0, 1.02)
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="upper left")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "p_sweep_collisions.png"))
    plt.close(fig)

    # ── 2. Transmission Delay vs p ────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    ax.plot(
        df["p_val"], df["avg_delay_ms"],
        marker="s", color=color_info, linewidth=2.4, markersize=7,
        label=f"Average Frame Delay (N={fixed_n})"
    )
    ax.set_title(f"p-Persistent CSMA: Transmission Delay vs p (Fixed N={fixed_n})", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Transmission Probability (p)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Average Transmission Delay (ms)", fontsize=11, fontweight="bold")
    ax.set_xlim(0.0, 1.02)
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="upper right")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "p_sweep_delay.png"))
    plt.close(fig)

    # ── 3. Throughput vs p ────────────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    ax.plot(
        df["p_val"], df["throughput_kbps"],
        marker="^", color=color_primary, linewidth=2.4, markersize=7,
        label=f"Throughput (N={fixed_n})"
    )
    # Highlight optimal p
    opt_row = df.loc[df["throughput_kbps"].idxmax()]
    ax.axvline(x=opt_row["p_val"], color="#F4A261", linestyle="--", alpha=0.85,
               label=f"Peak Throughput at p={opt_row['p_val']:.2f} ({opt_row['throughput_kbps']:.2f} kbps)")
    ax.set_title(f"p-Persistent CSMA: Throughput vs p (Fixed N={fixed_n})", fontsize=13, fontweight="bold", pad=12)
    ax.set_xlabel("Transmission Probability (p)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Throughput (kbps)", fontsize=11, fontweight="bold")
    ax.set_xlim(0.0, 1.02)
    ax.grid(True, linestyle="--", alpha=0.6)
    ax.legend(frameon=True, fontsize=10, loc="best")
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "p_sweep_throughput.png"))
    plt.close(fig)

    # ── 4. Combined 3-Panel Plot ──────────────────────────────────────────────
    fig, (ax1, ax2, ax3) = plt.subplots(1, 3, figsize=(18, 5), dpi=300)

    # (a) Collisions
    ax1.plot(df["p_val"], df["total_collisions"], marker="o", color=color_danger, linewidth=2.2, markersize=6)
    ax1.set_title("(a) Number of Collisions vs p", fontsize=12, fontweight="bold")
    ax1.set_xlabel("Probability (p)", fontsize=10, fontweight="bold")
    ax1.set_ylabel("Total Collisions", fontsize=10, fontweight="bold")
    ax1.set_xlim(0.0, 1.02)
    ax1.grid(True, linestyle="--", alpha=0.6)

    # (b) Transmission Delay
    ax2.plot(df["p_val"], df["avg_delay_ms"], marker="s", color=color_info, linewidth=2.2, markersize=6)
    ax2.set_title("(b) Transmission Delay vs p", fontsize=12, fontweight="bold")
    ax2.set_xlabel("Probability (p)", fontsize=10, fontweight="bold")
    ax2.set_ylabel("Avg Frame Delay (ms)", fontsize=10, fontweight="bold")
    ax2.set_xlim(0.0, 1.02)
    ax2.grid(True, linestyle="--", alpha=0.6)

    # (c) Throughput
    ax3.plot(df["p_val"], df["throughput_kbps"], marker="^", color=color_primary, linewidth=2.2, markersize=6)
    ax3.axvline(x=opt_row["p_val"], color="#F4A261", linestyle="--", alpha=0.8,
                label=f"Optimal p ≈ {opt_row['p_val']:.2f}")
    ax3.set_title("(c) Throughput vs p", fontsize=12, fontweight="bold")
    ax3.set_xlabel("Probability (p)", fontsize=10, fontweight="bold")
    ax3.set_ylabel("Throughput (kbps)", fontsize=10, fontweight="bold")
    ax3.set_xlim(0.0, 1.02)
    ax3.grid(True, linestyle="--", alpha=0.6)
    ax3.legend(frameon=True, fontsize=9)

    fig.suptitle(f"p-Persistent CSMA Performance as Functions of Probability p (Fixed N={fixed_n} Stations)",
                 fontsize=14, fontweight="bold", y=1.02)
    plt.tight_layout()
    fig.savefig(os.path.join(out_dir, "p_sweep_combined.png"), bbox_inches="tight")
    plt.close(fig)

    print(f"Generated p-sweep plots in: {out_dir}/")
    print(f"  - {os.path.join(out_dir, 'p_sweep_collisions.png')}")
    print(f"  - {os.path.join(out_dir, 'p_sweep_delay.png')}")
    print(f"  - {os.path.join(out_dir, 'p_sweep_throughput.png')}")
    print(f"  - {os.path.join(out_dir, 'p_sweep_combined.png')}")

if __name__ == "__main__":
    main()
