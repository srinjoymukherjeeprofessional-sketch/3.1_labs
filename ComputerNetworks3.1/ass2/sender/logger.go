// sender/logger.go
//
// ─── Sender Logging & Statistics ─────────────────────────────────────────────
//
// Theoretical / Experimental Overview:
// Precise per-frame tracking and aggregate metrics are essential to analyze
// ARQ protocol performance (throughput, retransmission overhead, and RTT adaptation).
//
// This module provides:
//   1. openCSV: Initializes the per-frame CSV log with headers:
//      [seq_idx, seq_num, elapsed_ms, rtt_ms, retransmitted]
//   2. logFrame: Writes a single row whenever a frame is acknowledged:
//      - seq_idx:       Absolute zero-indexed frame sequence number.
//      - seq_num:       Modulo-64 sequence number placed on the wire.
//      - elapsed_ms:    Time since simulation started.
//      - rtt_ms:        Round-trip time measured for this frame (or N/A).
//      - retransmitted: Boolean indicating if this frame required retransmission.
//   3. printSummary: Computes efficiency (Data / TotalSent * 100) and displays
//      a summary table, appending summary statistics to eval/results.csv.
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
	w.Write([]string{"seq_idx", "seq_num", "elapsed_ms", "rtt_ms", "retransmitted"})
	w.Flush()
	return f, w
}

// logFrame writes one row to the per-frame CSV log.
func (s *Sender) logFrame(seqIdx int, seqNum uint8, sentAt time.Time, ackAt time.Time, retx bool) {
	if s.csvW == nil {
		return
	}
	rttMs := "N/A"
	if !ackAt.IsZero() {
		rttMs = fmt.Sprintf("%.3f", ackAt.Sub(sentAt).Seconds()*1000)
	}
	s.csvW.Write([]string{
		strconv.Itoa(seqIdx),
		strconv.Itoa(int(seqNum)),
		fmt.Sprintf("%.3f", sentAt.Sub(s.startTime).Seconds()*1000),
		rttMs,
		strconv.FormatBool(retx),
	})
	s.csvW.Flush()
}

// printSummary prints end-of-run statistics to stdout and appends to results.csv.
func (s *Sender) printSummary(totalTime time.Duration) {
	efficiency := 0.0
	if s.framesSent > 0 {
		efficiency = float64(len(s.payloads)) / float64(s.framesSent) * 100
	}
	fmt.Printf("\n═══ Sender Summary ══════════════════════════\n")
	fmt.Printf("  Mode          : %s\n", s.cfg.Mode)
	fmt.Printf("  Window        : %d\n", s.cfg.WindowSize)
	fmt.Printf("  FCS           : %s\n", s.cfg.FCSAlgo)
	fmt.Printf("  Loss prob     : %.2f\n", s.cfg.LossProb)
	fmt.Printf("  Corrupt prob  : %.2f\n", s.cfg.CorruptProb)
	fmt.Printf("  Total frames  : %d (data: %d)\n", s.framesSent, len(s.payloads))
	fmt.Printf("  Retransmits   : %d\n", s.retransmits)
	fmt.Printf("  Efficiency    : %.1f%%\n", efficiency)
	fmt.Printf("  Total time    : %v\n", totalTime.Round(time.Millisecond))
	fmt.Printf("═════════════════════════════════════════════\n")

	// Append summary row to shared results.csv for eval plotting
	rf, err := os.OpenFile("eval/results.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer rf.Close()
		w := csv.NewWriter(rf)
		w.Write([]string{
			s.cfg.Mode,
			strconv.Itoa(s.cfg.WindowSize),
			s.cfg.FCSAlgo,
			fmt.Sprintf("%.2f", s.cfg.LossProb),
			fmt.Sprintf("%.2f", s.cfg.CorruptProb),
			strconv.Itoa(len(s.payloads)),
			strconv.Itoa(s.retransmits),
			fmt.Sprintf("%.2f", efficiency),
			fmt.Sprintf("%.0f", totalTime.Seconds()*1000),
		})
		w.Flush()
	}
}
