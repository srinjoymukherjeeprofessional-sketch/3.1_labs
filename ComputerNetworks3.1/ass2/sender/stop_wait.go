// sender/stop_wait.go
//
// ─── Stop-and-Wait ARQ Protocol (Sender) ──────────────────────────────────────
//
// Theoretical Overview:
// Stop-and-Wait is the simplest Automatic Repeat reQuest (ARQ) protocol.
// The sender transmits exactly one data frame and then stops, waiting for a
// positive acknowledgement (ACK) from the receiver before sending the next frame.
//
// Key Characteristics:
//   - Window Size: W = 1 (only 1 unacknowledged frame allowed in flight).
//   - Sequence Numbers: 0 and 1 (alternating bit protocol), though in our
//     implementation we use modulo-64 to maintain uniform frame structure.
//   - Retransmission Timer: If the ACK does not arrive within the adaptive
//     Jacobson/Karels Retransmission Timeout (RTO), or if a corrupted ACK is
//     received, the sender retransmits the same frame and backs off RTO.
//   - RTT Adaptation: On receiving a valid, timely ACK, measured RTT updates
//     the smoothed RTT (SRTT) and variance (RTTVAR) to adapt to network delay.
//   - Flow Control: Inherent in the protocol; sender never overwhelms receiver.
package main

import (
	"fmt"
	"time"

	"ass2/utils"
)

// RunStopWait implements the sender side of Stop-and-Wait ARQ.
//
// Workflow per frame:
//  1. Construct frame with sequence number (idx % SeqModulus).
//  2. Transmit frame through simulated channel (which may drop/delay/corrupt it).
//  3. Start timeout countdown set to current adaptive RTO.
//  4. If channel dropped frame or timeout expires or corrupted ACK arrives:
//     - Double RTO (exponential backoff).
//     - Retransmit the frame and retry.
//  5. If valid ACK matching sequence number arrives:
//     - Update RTT estimate and RTO.
//     - Advance to the next frame.
//  6. Once all payload chunks are ACKed, send EOF sentinel to close gracefully.
func (s *Sender) RunStopWait() {
	fmt.Println("[Stop-Wait] Starting transmission...")

	for idx, payload := range s.payloads {
		seqNum := uint8(idx % SeqModulus)
		frame := s.Framing(payload, seqNum, utils.TypeData)
		retx := false

		for {
			sentAt := time.Now()
			dropped, err := s.Send(frame)
			if err != nil {
				fmt.Printf("[Stop-Wait] Send error on frame %d: %v\n", idx, err)
				return
			}
			if dropped {
				fmt.Printf("[Stop-Wait] Frame %d (seq %d) dropped by channel — retransmitting\n", idx, seqNum)
				s.rto.Backoff()
				retx = true
				s.mu.Lock()
				s.retransmits++
				s.mu.Unlock()
				time.Sleep(s.rto.Get()) // Wait before retry
				continue
			}

			// Wait for ACK with timeout using TCP deadline
			s.conn.SetDeadline(time.Now().Add(s.rto.Get()))
			ackFrame, err := s.Recv()
			s.conn.SetDeadline(time.Time{}) // Clear deadline

			if err != nil {
				// Timeout (or connection error) — retransmit
				fmt.Printf("[Stop-Wait] Timeout on frame %d (seq %d), RTO=%.0fms — retransmitting\n",
					idx, seqNum, float64(s.rto.Get().Milliseconds()))
				s.rto.Backoff()
				retx = true
				s.mu.Lock()
				s.retransmits++
				s.mu.Unlock()
				continue
			}
			if ackFrame == nil {
				// Corrupted ACK — treat as missing ACK, retransmit
				fmt.Printf("[Stop-Wait] Corrupted ACK for frame %d — retransmitting\n", idx)
				retx = true
				s.mu.Lock()
				s.retransmits++
				s.mu.Unlock()
				continue
			}
			if ackFrame.Header.FrameType == utils.TypeACK && ackFrame.Header.SeqNum == seqNum {
				// Valid ACK — update RTT estimate and move forward
				s.Timeout(sentAt)
				s.logFrame(idx, seqNum, sentAt, time.Now(), retx)
				fmt.Printf("[Stop-Wait] Frame %d (seq %d) ACKed, RTT=%.1fms, RTO=%.0fms\n",
					idx, seqNum,
					time.Since(sentAt).Seconds()*1000,
					float64(s.rto.Get().Milliseconds()))
				break
			}
		}
	}
}
