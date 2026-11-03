// sender/gbn.go
//
// ─── Go-Back-N ARQ Protocol (Sender) ──────────────────────────────────────────
//
// Theoretical Overview:
// Go-Back-N (GBN) is a sliding window protocol that allows the sender to transmit
// multiple frames (up to window size N) before needing an acknowledgement.
// This significantly improves channel utilization over Stop-and-Wait on high
// bandwidth-delay product (BDP) links.
//
// Key Characteristics:
//   - Sender Window: size N (in-flight frames in range [base, base+N)).
//   - Cumulative ACKs: An ACK with sequence number k acknowledges all frames up
//     to and including k. If ACK(k) is lost but ACK(k+1) arrives, frame k is
//     still confirmed!
//   - Single Timer: The sender runs a single timer for the oldest unacknowledged
//     frame (base). Whenever the base advances, the timer is restarted.
//   - Go-Back-N Rule on Timeout: When the timer expires, the sender assumes
//     frame `base` was lost, and retransmits the ENTIRE outstanding window
//     from `base` up to `next-1`. Even if subsequent frames reached the receiver,
//     the receiver discarded them because its receive window is 1.
package main

import (
	"fmt"
	"sync"
	"time"

	"ass2/utils"
)

// RunGBN implements Go-Back-N ARQ with window size N.
//
// Workflow:
//  1. Reader Goroutine: reads incoming ACKs from the TCP socket asynchronously.
//     Valid TypeACKs are pushed to ackCh. EOF ACK closes errCh.
//  2. Window Management:
//     - base: absolute index of the oldest unACKed frame.
//     - next: absolute index of the next frame to be transmitted.
//     - While next < base + N and next < total, frames are transmitted.
//  3. ACK Processing:
//     - Cumulative search: finds the highest frame in [base, next) matching ackSeq.
//     - Advances base, measures RTT for ACKed frames, and resets the timer.
//  4. Timer Expiry:
//     - Retransmits all frames in [base, next).
//     - Backs off RTO exponentially.
//     - Resets the timer.
//  5. On completion, transmits EOF sentinel until acknowledged.
func (s *Sender) RunGBN(n int) {
	fmt.Printf("[GBN] Starting transmission (window=%d)...\n", n)
	total := len(s.payloads)

	// base: absolute index of oldest unACKed frame
	// next: absolute index of next frame yet to be sent
	base, next := 0, 0

	// Sent times keyed by absolute frame index (for RTT measurement)
	sentAt := make(map[int]time.Time)
	var wmu sync.Mutex // protects base, next, sentAt

	// Channel carrying decoded ACK sequence numbers from the reader goroutine
	ackCh := make(chan uint8, n*4)
	errCh := make(chan error, 1)

	// Goroutine: continuously read frames from TCP and forward ACKs/EOF-ACKs
	go func() {
		for {
			s.conn.SetDeadline(time.Time{}) // no deadline on the reader itself
			raw, err := utils.ReadACKFrame(s.conn)
			if err != nil {
				errCh <- err
				return
			}
			f, err := utils.Deserialize(raw, s.cfg.FCSAlgo)
			if err != nil || f == nil {
				continue // corrupted ACK — ignore
			}
			if f.Header.FrameType == utils.TypeACK {
				ackCh <- f.Header.SeqNum
			}
		}
	}()

	// Single timer for the base frame; reset whenever base advances
	timer := time.NewTimer(s.rto.Get())
	defer timer.Stop()

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(s.rto.Get())
	}

	for {
		wmu.Lock()
		done := base >= total
		wmu.Unlock()
		if done {
			break
		}

		// Fill the window: send as many frames as the window allows
		wmu.Lock()
		for next < base+n && next < total {
			seqNum := uint8(next % SeqModulus)
			frame := s.Framing(s.payloads[next], seqNum, utils.TypeData)
			now := time.Now()
			dropped, err := s.Send(frame)
			if err != nil {
				wmu.Unlock()
				fmt.Printf("[GBN] Send error: %v\n", err)
				return
			}
			if !dropped {
				sentAt[next] = now
			}
			if next == base {
				resetTimer() // start timer when the base frame is (re)sent
			}
			next++
		}
		wmu.Unlock()

		// Wait for an ACK or a timeout
		select {
		case ackSeq := <-ackCh:
			wmu.Lock()
			// Cumulative ACK: advance base to the frame AFTER the ACKed one.
			// Find the highest index i in [base, next) where i%64 == ackSeq.
			for i := base; i < next; i++ {
				if uint8(i%SeqModulus) == ackSeq {
					// All frames up to and including i are implicitly ACKed
					for j := base; j <= i; j++ {
						if t, ok := sentAt[j]; ok {
							s.Timeout(t)
							s.logFrame(j, uint8(j%SeqModulus), t, time.Now(), j != base)
							delete(sentAt, j)
						}
					}
					base = i + 1
					fmt.Printf("[GBN] Cumulative ACK seq=%d → base now %d, RTO=%.0fms\n",
						ackSeq, base, float64(s.rto.Get().Milliseconds()))
					break
				}
			}
			if base < total {
				resetTimer()
			}
			wmu.Unlock()

		case <-timer.C:
			// Timeout — retransmit the entire outstanding window (GBN rule)
			wmu.Lock()
			fmt.Printf("[GBN] Timeout on base=%d (seq %d), retransmitting window [%d, %d)\n",
				base, uint8(base%SeqModulus), base, next)
			s.rto.Backoff()
			for i := base; i < next; i++ {
				seqNum := uint8(i % SeqModulus)
				frame := s.Framing(s.payloads[i], seqNum, utils.TypeData)
				now := time.Now()
				dropped, _ := s.Send(frame)
				if !dropped {
					sentAt[i] = now
				}
				s.mu.Lock()
				s.retransmits++
				s.mu.Unlock()
			}
			resetTimer()
			wmu.Unlock()

		case err := <-errCh:
			if err != nil {
				fmt.Printf("[GBN] Connection error: %v\n", err)
			}
			return
		}
	}

	// Unblock and terminate the background reader goroutine
	s.conn.SetDeadline(time.Now())
	<-errCh
	s.conn.SetDeadline(time.Time{})
}
