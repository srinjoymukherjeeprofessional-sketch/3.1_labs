// receiver/sr.go
//
// ─── Selective Repeat ARQ Protocol (Receiver) ─────────────────────────────────
//
// Theoretical Overview:
// Unlike Go-Back-N, Selective Repeat allows the receiver to accept and buffer
// frames that arrive out of order, provided they fall within the receiver window
// [base, base + N).
//
// Key Characteristics & Rules:
//   - Receive Window: W_r = N. Window size must satisfy N <= SeqModulus / 2.
//   - Out-of-Order Buffering: If an error-free frame arrives out of order but
//     inside the receive window, the receiver buffers it and immediately sends an
//     individual ACK for that frame.
//   - Window Advancement: The window base slides forward only when the frame
//     at `base` is received, delivering all consecutive buffered frames in-order
//     to the output file.
//   - Duplicate / Out-of-Window Frames: If a frame arrives with a sequence number
//     outside the window (which typically means a previously ACKed frame whose
//     ACK was lost or delayed), the receiver re-sends an ACK for that frame so
//     the sender can unblock, but does NOT re-deliver it to the output file.
//   - NAK Fast Retransmission (Optional): When a corrupted frame (FCS failure)
//     is detected and NAK is enabled:
//     If the sequence number can still be extracted from the control byte and
//     falls within the window, the receiver transmits a Negative Acknowledgement
//     (NAK) immediately. This allows the sender to retransmit the lost frame
//     without waiting for its timer to expire.
package main

import (
	"fmt"

	"ass2/utils"
)

// RunSR implements the receiver side of Selective Repeat ARQ.
//
// Workflow:
//  1. Buffer ring: a circular buffer of size 64 stores out-of-order frames.
//  2. If a valid frame falls within [base, base+N) and hasn't been received:
//     - Buffer the payload.
//     - Send individual ACK for this sequence number.
//     - Slide `base` forward over consecutive buffered frames, writing them to disk.
//  3. If a frame falls below the window (duplicate):
//     - Re-send ACK to allow sender window to advance.
//  4. If frame is corrupted (FCS mismatch):
//     - If NAK is enabled, send NAK for the corrupted sequence number.
//  5. On EOF: acknowledge and exit cleanly.
func (r *Receiver) RunSR(n int, nakEnabled bool) {
	fmt.Printf("[SR] Listening for frames (window=%d, NAK=%v)...\n", n, nakEnabled)

	// base: absolute index of oldest undelivered frame
	base := 0
	// buffer holds received out-of-order frames indexed by seqNum (ring of size 64)
	type bufEntry struct {
		payload  []byte
		frameIdx int
		received bool
	}
	buf := make([]bufEntry, SeqModulus)

	// outIdx tracks absolute frame index for logging
	outIdx := 0

	// isInWindow checks whether a received seq falls within [base, base+N)
	isInWindow := func(seq uint8) bool {
		for i := 0; i < n; i++ {
			if uint8((base+i)%SeqModulus) == seq {
				return true
			}
		}
		return false
	}

	for {
		frame, raw, err := r.Recv()
		if err != nil {
			fmt.Println("[SR] Sender finished — connection closed.")
			return
		}

		if frame == nil {
			// Corrupted frame — try to extract seq from raw control byte for NAK
			r.discarded++
			if nakEnabled && len(raw) > 14 {
				corruptedSeq := raw[14] & 0x3F
				if isInWindow(corruptedSeq) {
					fmt.Printf("[SR] Corrupted frame seq~%d — sending NAK\n", corruptedSeq)
					r.Send(utils.NewNAKFrame(ReceiverMAC, SenderMAC, corruptedSeq))
					r.naksSent++
				}
			}
			continue
		}

		seq := frame.Header.SeqNum

		if isInWindow(seq) && !buf[seq].received {
			// Frame is within receive window and not yet buffered — accept it
			absIdx := base
			for uint8(absIdx%SeqModulus) != seq {
				absIdx++
			}
			buf[seq] = bufEntry{
				payload:  frame.Payload,
				frameIdx: absIdx,
				received: true,
			}
			r.accepted++
			r.logFrame(absIdx, seq, true, false)
			fmt.Printf("[SR] Buffered frame idx=%d (seq %d)\n", absIdx, seq)

			// Send individual ACK for this frame
			r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, seq))

			// Deliver consecutive frames from base forward
			for buf[uint8(base%SeqModulus)].received {
				slot := uint8(base % SeqModulus)
				r.writePayload(buf[slot].payload)
				fmt.Printf("[SR] Delivered frame %d (seq %d) to output\n", outIdx, slot)
				buf[slot] = bufEntry{} // clear slot
				base++
				outIdx++
			}
		} else if !isInWindow(seq) {
			// Out-of-window duplicate — re-ACK to help sender advance (already delivered)
			fmt.Printf("[SR] Duplicate/out-of-window seq=%d — re-ACKing\n", seq)
			r.Send(utils.NewACKFrame(ReceiverMAC, SenderMAC, seq))
		}
		// If already buffered (duplicate within window), ignore
	}
}
