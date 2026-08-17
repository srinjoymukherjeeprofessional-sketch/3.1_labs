"""Create packet-level tables and graphs from the end-to-end evaluation."""

from pathlib import Path

import matplotlib.pyplot as plt
import pandas as pd


PACKET_FILE = Path("evaluation_packets.csv")
RUN_FILE = Path("evaluation_end_to_end.csv")


def payload_from_hex(packet_hex):
    try:
        raw = bytes.fromhex(packet_hex)
        header = int.from_bytes(raw[12:16], byteorder="big")
        payload_length = header & 0x00FFFFFF
        return raw[16:16 + payload_length].decode("utf-8", errors="replace")
    except (ValueError, TypeError):
        return ""


def main():
    if not PACKET_FILE.exists():
        raise SystemExit(
            "evaluation_packets.csv is missing. Rebuild and rerun "
            "evaluation_end_to_end first."
        )

    packets = pd.read_csv(PACKET_FILE)
    runs = pd.read_csv(RUN_FILE) if RUN_FILE.exists() else pd.DataFrame()
    schemes = ["checksum", "crc10", "crc16", "crc32", "crc8"]

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
    details = packets[
        ["case_id", "packet_index", "error_start", "error_end",
         "error_length", "error_region", "packet_hex"]
    ].drop_duplicates(["case_id", "packet_index"])
    paired = paired.merge(details, on=["case_id", "packet_index"], how="left")
    paired["payload_text"] = paired["packet_hex"].map(payload_from_hex)

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
    paired[paired["error_mode"] != "clean"].to_csv("packet_comparison.csv", index=False)

    # Isolate only packets missed by at least one scheme.
    failures = paired[
        (paired["error_mode"] != "clean") & (paired["failed_by"] != "")
    ].copy()
    failures["packet_error"] = failures.apply(
        lambda row: (
            f"case {int(row.case_id)}, packet {int(row.packet_index)} "
            f"[{int(row.error_start)}-{int(row.error_end)}]"
        ),
        axis=1,
    )
    failures.to_csv("failed_packets.csv", index=False)
    with open("failed_packets_report.txt", "w", encoding="utf-8") as report:
        for _, row in failures.iterrows():
            report.write(
                f"Case {int(row.case_id)}, packet {int(row.packet_index)}\n"
                f"Error: bytes {int(row.error_start)}-{int(row.error_end)} "
                f"({row.error_region}), length {int(row.error_length)}\n"
                f"Actual packet hex:\n{row.packet_hex}\n"
                f"Payload text:\n{row.payload_text}\n"
                f"Detected by: {row.detected_by or 'none'}\n"
                f"Failed by: {row.failed_by or 'none'}\n"
            )
            for scheme in schemes:
                report.write(
                    f"{scheme}: {'DETECTED' if row[f'detected_{scheme}'] else 'FAILED'}, "
                    f"{row[f'time_{scheme}']} ns\n"
                )
            report.write("\n" + "-" * 80 + "\n\n")

    # Table output for quick inspection.
    print("\nScheme summary:\n")
    print(summary.to_string(index=False, float_format=lambda x: f"{x:.2f}"))
    print("\nPaired packet comparison sample:\n")
    print(
        paired[paired["error_mode"] != "clean"]
        [["case_id", "packet_index", "error_mode", "error_start", "error_end",
          "error_region", "detected_by", "failed_by"]]
        .head(20)
        .to_string(index=False)
    )
    print(f"\nPackets missed by at least one scheme: {len(failures)}")
    if not failures.empty:
        print(
            failures[
                ["packet_error", "error_mode", "error_region", "detected_by", "failed_by"]
            ].to_string(index=False)
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

    # Failure-only heatmap. Every row is an exact shared packet-error.
    if not failures.empty:
        failure_matrix = failures.set_index("packet_error")[
            [f"detected_{scheme}" for scheme in schemes]
        ].astype(int)
        plt.figure(figsize=(10, max(5, min(18, 0.28 * len(failure_matrix)))))
        plt.imshow(
            failure_matrix.to_numpy(),
            aspect="auto",
            cmap="RdYlGn",
            vmin=0,
            vmax=1,
        )
        plt.xticks(range(len(schemes)), schemes)
        if len(failure_matrix) <= 60:
            plt.yticks(range(len(failure_matrix)), failure_matrix.index, fontsize=7)
        else:
            plt.ylabel("Isolated packet-error rows")
        plt.xlabel("Scheme")
        plt.title("Exact packet-errors missed by at least one scheme")
        plt.colorbar(label="1 = detected, 0 = failed")
        plt.tight_layout()
        plt.savefig("failed_packets_heatmap.png", dpi=180)
        plt.close()

        failure_counts = {
            scheme: int((failures[f"detected_{scheme}"] == 0).sum())
            for scheme in schemes
        }
        plt.figure(figsize=(8, 5))
        plt.bar(failure_counts.keys(), failure_counts.values())
        plt.ylabel("Failed packet-errors")
        plt.title("Failure count on isolated packet-errors")
        plt.tight_layout()
        plt.savefig("failed_packets_by_scheme.png", dpi=180)
        plt.close()

    print("\nCreated:")
    print("  scheme_summary.csv")
    print("  packet_comparison.csv")
    print("  failed_packets.csv")
    print("  failed_packets_report.txt")
    print("  detection_rate.png")
    print("  missed_errors_by_type.png")
    print("  validation_time_boxplot.png")
    print("  paired_packet_detection_heatmap.png")
    if not failures.empty:
        print("  failed_packets_heatmap.png")
        print("  failed_packets_by_scheme.png")


if __name__ == "__main__":
    main()
