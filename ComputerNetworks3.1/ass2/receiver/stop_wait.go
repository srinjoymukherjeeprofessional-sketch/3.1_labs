// receiver/stop_wait.go
//
// ─── Stop-and-Wait ARQ Protocol (Receiver) ────────────────────────────────────
//
// Theoretical Overview:
// In Stop-and-Wait ARQ, the receiver expects frames strictly in sequential order.
// Since window size is 1, the receiver maintains a single variable: `expected`,
// representing the sequence number of the next frame it will accept.
//
// Key Characteristics & Rules:
//   - In-Order Frame: If `frame.SeqNum == expected`, the payload is accepted,
//     appended to the output file, logged, and an ACK with that sequence number
//     is transmitted back to the sender. `expected` is advanced modulo 64.
//   - Duplicate Frame (Lost ACK scenario): If a frame arrives with an unexpected
//     sequence number (e.g. sender timed out because the receiver's previous
//     ACK was lost or delayed), the receiver does NOT write duplicate data to the
//     output file. Instead, it re-transmits the last sent ACK so the sender can
//     unblock and advance.
//   - Corrupted Frame: If FCS verification fails, the corrupted frame is silently
//     discarded. The sender will eventually time out and retransmit.
//   - EOF Sentinel: On receiving TypeEOF, the receiver sends an ACK and terminates.
package main

import (
	"fmt"

	"ass2/utils"
)

// RunStopWait implements the receiver side of Stop-and-Wait ARQ.
//
// Expected sequence numbers alternate 0, 1, 2, … (mod 64).
// A duplicate causes the last ACK to be resent — this handles the case where
// the first ACK was lost in transit.
func (r *Receiver) RunStopWait() {
	fmt.Println("[Stop-Wait] Listening for frames...")
	expected := uint8(0)
	frameIdx := 0

	for {
		frame, raw, err := r.Recv()
		if err != nil {
			fmt.Println("[Stop-Wait] Sender finished — connection closed.")
			return
		}

		if frame == nil {
			// Corrupted frame — discard silently (sender will timeout)
			fmt.Printf("[Stop-Wait] Corrupted frame (FCS mismatch) — discarded\n")
			r.discarded++
			_ = raw // raw bytes available if needed
			continue
		}

		seq := frame.Header.SeqNum

		if seq == expected {
			// In-order, valid frame — accept and ACK
			r.writePayload(frame.Payload)
			r.accepted++
			r.logFrame(frameIdx, seq, true, false)
			fmt.Printf("[Stop-Wait] Frame %d (seq %d) accepted\n", frameIdx, seq)

			ack := utils.NewACKFrame(ReceiverMAC, SenderMAC, seq)
			r.Send(ack)

			expected = (expected + 1) % SeqModulus
			frameIdx++
		} else {
			// Duplicate or out-of-order: resend the last ACK
			lastACKSeq := uint8(0)
			if expected > 0 {
				lastACKSeq = expected - 1
			} else {
				lastACKSeq = SeqModulus - 1
			}
			fmt.Printf("[Stop-Wait] Out-of-order seq=%d (expected %d) — resending ACK %d\n",
				seq, expected, lastACKSeq)
			r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, lastACKSeq))
		}
	}
}
