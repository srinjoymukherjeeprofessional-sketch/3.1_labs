"""Create packet-level tables and graphs from the end-to-end evaluation."""

from pathlib import Path

import matplotlib.pyplot as plt
import pandas as pd


PACKET_FILE = Path("evaluation_packets.csv")
RUN_FILE = Path("evaluation_end_to_end.csv")


def main():
    if not PACKET_FILE.exists():
        raise SystemExit(
            "evaluation_packets.csv is missing. Rebuild and rerun "
            "evaluation_end_to_end first."
        )

    packets = pd.read_csv(PACKET_FILE)
    runs = pd.read_csv(RUN_FILE) if RUN_FILE.exists() else pd.DataFrame()
    schemes = ["checksum", "crc10", "crc16", "crc32"]

    if packets.empty or "validation_time_ns" not in packets.columns:
        raise SystemExit(
            "evaluation_packets.csv has no packet results. Rebuild receiver "
            "from the updated receiver.c, rebuild evaluation_end_to_end, and "
            "rerun the end-to-end evaluation. The receiver must emit EVAL_RESULT lines."
        )

    # Every row here is one packet processed by the real receiver.
    errors = packets[packets["error_mode"] != "clean"].copy()
    summary = (
        packets.groupby("scheme")
        .agg(
            packets=("detected", "size"),
            detected_packets=("detected", "sum"),
            invalid_metadata_packets=("invalid_metadata", "sum"),
            average_validation_ns=("validation_time_ns", "mean"),
            median_validation_ns=("validation_time_ns", "median"),
            p95_validation_ns=("validation_time_ns", lambda x: x.quantile(0.95)),
        )
        .reset_index()
    )
    error_summary = (
        errors.groupby("scheme")
        .agg(error_packets=("detected", "size"), detected=("detected", "sum"))
        .reset_index()
    )
    error_summary["missed"] = error_summary["error_packets"] - error_summary["detected"]
    error_summary["detection_rate_percent"] = (
        100 * error_summary["detected"] / error_summary["error_packets"]
    )
    summary = summary.merge(error_summary, on="scheme", how="left")

    if not runs.empty:
        run_summary = (
            runs.groupby("scheme")
            .agg(
                average_sender_wall_ms=("wall_ms", "mean"),
                average_sender_user_cpu_ms=("user_cpu_ms", "mean"),
                average_sender_system_cpu_ms=("system_cpu_ms", "mean"),
                average_sender_rss_kb=("max_rss_kb", "mean"),
            )
            .reset_index()
        )
        summary = summary.merge(run_summary, on="scheme", how="left")

    summary.to_csv("scheme_summary.csv", index=False)

    # Paired comparison: same case, same packet index, all schemes side by side.
    detected = packets.pivot_table(
        index=["case_id", "packet_index", "seed", "error_mode", "burst_length"],
        columns="scheme",
        values="detected",
        aggfunc="first",
    ).reset_index()
    times = packets.pivot_table(
        index=["case_id", "packet_index"],
        columns="scheme",
        values="validation_time_ns",
        aggfunc="first",
    ).reset_index()
    detected.columns = [
        c if c in {"case_id", "packet_index", "seed", "error_mode", "burst_length"}
        else f"detected_{c}"
        for c in detected.columns
    ]
    times.columns = [
        "time_" + str(c) if c not in {"case_id", "packet_index"} else c
        for c in times.columns
    ]
    paired = detected.merge(times, on=["case_id", "packet_index"], how="left")

    detected_cols = [f"detected_{scheme}" for scheme in schemes]
    for col in detected_cols:
        if col not in paired:
            paired[col] = pd.NA
    paired["detected_by"] = paired.apply(
        lambda row: ",".join(s for s in schemes if row[f"detected_{s}"] == 1), axis=1
    )
    paired["failed_by"] = paired.apply(
        lambda row: ",".join(s for s in schemes if row[f"detected_{s}"] == 0), axis=1
    )
    paired[paired["error_mode"] != "clean"].to_csv(
        "packet_comparison.csv", index=False
    )

    # Table output for quick inspection.
    print("\nScheme summary:\n")
    print(summary.to_string(index=False, float_format=lambda x: f"{x:.2f}"))
    print("\nPaired packet comparison sample:\n")
    print(
        paired[paired["error_mode"] != "clean"]
        [["case_id", "packet_index", "error_mode", "burst_length", "detected_by", "failed_by"]]
        .head(20)
        .to_string(index=False)
    )

    plt.style.use("seaborn-v0_8-whitegrid")

    # Detection-rate comparison.
    plt.figure(figsize=(8, 5))
    plt.bar(error_summary["scheme"], error_summary["detection_rate_percent"])
    plt.ylim(0, 100.5)
    plt.ylabel("Detected error packets (%)")
    plt.title("Error-detection rate by scheme")
    plt.tight_layout()
    plt.savefig("detection_rate.png", dpi=180)
    plt.close()

    # Missed-error comparison by error type.
    by_mode = (
        errors.groupby(["error_mode", "scheme"])
        .agg(error_packets=("detected", "size"), detected=("detected", "sum"))
        .reset_index()
    )
    by_mode["missed"] = by_mode["error_packets"] - by_mode["detected"]
    by_mode.pivot(index="error_mode", columns="scheme", values="missed")[schemes].plot(
        kind="bar", figsize=(9, 5)
    )
    plt.ylabel("Missed error packets")
    plt.title("Missed errors by error type")
    plt.xticks(rotation=0)
    plt.tight_layout()
    plt.savefig("missed_errors_by_type.png", dpi=180)
    plt.close()

    # Validation-time distributions.
    plt.figure(figsize=(9, 5))
    data = [packets.loc[packets["scheme"] == s, "validation_time_ns"] for s in schemes]
    plt.boxplot(data, label=schemes, showfliers=False)
    plt.ylabel("Receiver validation time (ns)")
    plt.title("Validation-time distribution")
    plt.tight_layout()
    plt.savefig("validation_time_boxplot.png", dpi=180)
    plt.close()

    # Same-packet heatmap: rows are paired error packets, columns are schemes.
    heat = paired[paired["error_mode"] != "clean"][detected_cols].fillna(0).astype(int)
    plt.figure(figsize=(8, 8))
    plt.imshow(heat.to_numpy(), aspect="auto", cmap="RdYlGn", vmin=0, vmax=1)
    plt.xticks(range(len(schemes)), schemes)
    plt.ylabel("Same-error packet cases")
    plt.title("Detection of each identical error by each scheme")
    plt.colorbar(label="1 = detected, 0 = missed")
    plt.tight_layout()
    plt.savefig("paired_packet_detection_heatmap.png", dpi=180)
    plt.close()

    print("\nCreated:")
    print("  scheme_summary.csv")
    print("  packet_comparison.csv")
    print("  detection_rate.png")
    print("  missed_errors_by_type.png")
    print("  validation_time_boxplot.png")
    print("  paired_packet_detection_heatmap.png")


if __name__ == "__main__":
    main()
