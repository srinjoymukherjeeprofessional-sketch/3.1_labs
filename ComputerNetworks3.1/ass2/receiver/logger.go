// receiver/logger.go
//
// ─── Receiver Logging & Statistics ───────────────────────────────────────────
//
// Theoretical / Experimental Overview:
// Receiver logs allow comparing frames received against sender transmission records,
// identifying discards (due to FCS corruption or out-of-order delivery) and tracking
// NAK events.
//
// This module provides:
//   1. openCSV: Initializes the per-frame CSV log with headers:
//      [seq_idx, seq_num, elapsed_ms, accepted, nak_sent]
//   2. logFrame: Writes a single row for each processed frame event:
//      - seq_idx:    Zero-indexed frame count seen by the receiver.
//      - seq_num:    Modulo-64 sequence number read from the frame header.
//      - elapsed_ms: Time since the receiver began listening.
//      - accepted:   True if the frame passed FCS and in-order validation.
//      - nak_sent:   True if an immediate negative acknowledgement was issued.
//   3. printSummary: Outputs aggregate counts of accepted, discarded, and NAKed frames.
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

// openCSV creates (or truncates) a CSV log file and writes the header row.
func openCSV(prefix, mode string, loss float64) (*os.File, *csv.Writer) {
	os.MkdirAll("logs", 0755)
	name := fmt.Sprintf("logs/%s_%s_loss%.1f.csv", prefix, mode, loss)
	f, err := os.Create(name)
	if err != nil {
		return nil, nil
	}
	w := csv.NewWriter(f)
	w.Write([]string{"seq_idx", "seq_num", "elapsed_ms", "accepted", "nak_sent"})
	w.Flush()
	return f, w
}

// logFrame writes a row to the per-frame receiver CSV log.
func (r *Receiver) logFrame(seqIdx int, seqNum uint8, accepted bool, nakSent bool) {
	if r.csvW == nil {
		return
	}
	r.csvW.Write([]string{
		strconv.Itoa(seqIdx),
		strconv.Itoa(int(seqNum)),
		fmt.Sprintf("%.3f", time.Since(r.startTime).Seconds()*1000),
		strconv.FormatBool(accepted),
		strconv.FormatBool(nakSent),
	})
	r.csvW.Flush()
}

// printSummary prints reception statistics.
func (r *Receiver) printSummary() {
	fmt.Printf("\n═══ Receiver Summary ════════════════════════\n")
	fmt.Printf("  Mode          : %s\n", r.cfg.Mode)
	fmt.Printf("  FCS           : %s\n", r.cfg.FCSAlgo)
	fmt.Printf("  Frames accepted : %d\n", r.accepted)
	fmt.Printf("  Frames discarded: %d\n", r.discarded)
	fmt.Printf("  NAKs sent       : %d\n", r.naksSent)
	fmt.Printf("  Output file     : %s\n", r.cfg.OutputFile)
	fmt.Printf("═════════════════════════════════════════════\n")
}
