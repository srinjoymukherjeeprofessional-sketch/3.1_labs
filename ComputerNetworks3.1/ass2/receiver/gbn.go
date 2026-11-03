// receiver/gbn.go
//
// ─── Go-Back-N ARQ Protocol (Receiver) ────────────────────────────────────────
//
// Theoretical Overview:
// In Go-Back-N ARQ, the receiver has a fixed window size of 1. It only accepts
// frames that arrive in strict sequential order.
//
// Key Characteristics & Rules:
//   - Window Size: W_r = 1.
//   - Strict Sequential Acceptance: Only `frame.SeqNum == expected` is accepted.
//     Any frame arriving out of order (e.g. frame k arrived, but frame k-1 was
//     dropped or delayed) is strictly discarded. The receiver does NOT buffer
//     out-of-order frames in GBN.
//   - Cumulative ACKs: When an in-order frame is accepted, the receiver sends an
//     ACK with that sequence number.
//   - Handling Discarded / Corrupted Frames: Whenever an out-of-order or
//     corrupted frame arrives, the receiver discards it and re-transmits an ACK
//     for the LAST successfully received in-order frame (`lastACK`). This tells
//     the sender where the receiver is stuck so the sender can retransmit on timeout.
//   - Graceful EOF: When TypeEOF is received, sends an ACK and finishes.
package main

import (
	"fmt"

	"ass2/utils"
)

// RunGBN implements the receiver side of Go-Back-N ARQ.
//
// The receiver window size is 1: it only accepts frames in strict sequential order.
// Out-of-order and corrupted frames are silently discarded. The cumulative ACK
// always reflects the last successfully received in-order frame.
func (r *Receiver) RunGBN(n int) {
	fmt.Println("[GBN] Listening for frames...")
	expected := uint8(0)
	frameIdx := 0
	lastACK := uint8(SeqModulus - 1) // the seq of the last sent ACK (initially none)

	for {
		frame, _, err := r.Recv()
		if err != nil {
			fmt.Println("[GBN] Sender finished — connection closed.")
			return
		}

		if frame == nil {
			// Corrupted frame — discard (no ACK sent; sender retransmits on timeout)
			r.discarded++
			fmt.Printf("[GBN] Corrupted frame discarded, re-ACKing seq %d\n", lastACK)
			// Resend last good ACK so sender knows we haven't moved forward
			if frameIdx > 0 {
				r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, lastACK))
			}
			continue
		}

		seq := frame.Header.SeqNum

		if seq == expected {
			r.writePayload(frame.Payload)
			r.accepted++
			r.logFrame(frameIdx, seq, true, false)
			fmt.Printf("[GBN] Frame %d (seq %d) accepted\n", frameIdx, seq)

			// Cumulative ACK: acknowledge this seq
			r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, seq))
			lastACK = seq
			expected = (expected + 1) % SeqModulus
			frameIdx++
		} else {
			// Out-of-order — discard and resend last in-order ACK (GBN rule)
			r.discarded++
			fmt.Printf("[GBN] Out-of-order seq=%d (expected %d) — discarding, re-ACKing %d\n",
				seq, expected, lastACK)
			if frameIdx > 0 {
				r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, lastACK))
			}
		}
	}
}
