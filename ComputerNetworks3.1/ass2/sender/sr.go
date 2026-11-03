// sender/sr.go
//
// ─── Selective Repeat ARQ Protocol (Sender) ───────────────────────────────────
//
// Theoretical Overview:
// Selective Repeat (SR) avoids the retransmission overhead of Go-Back-N by
// retransmitting ONLY the frames that were lost or corrupted, rather than the
// entire window.
//
// Key Characteristics:
//   - Sender & Receiver Windows: both have size N. Window size must satisfy
//     N <= SeqModulus / 2 (e.g. N <= 32 for mod 64) to prevent sequence number
//     ambiguity between old duplicates and new frames.
//   - Individual ACKs: Each successfully received frame is acknowledged
//     individually with its exact sequence number.
//   - Per-Frame Timers: Every outstanding frame has its own independent timer.
//     When a frame's timer expires, ONLY that specific frame is retransmitted.
//   - Sliding Window: The base advances only when the oldest unACKed frame is
//     finally acknowledged, skipping over any subsequent frames that were
//     already received and acknowledged out of order.
//   - NAK Fast Retransmit (Optional): When the receiver detects corruption via
//     FCS mismatch, it can immediately send a Negative Acknowledgement (NAK)
//     for that sequence number. The sender retransmits the frame immediately
//     without waiting for the per-frame timer to expire, drastically reducing
//     recovery latency.
package main

import (
	"fmt"
	"sync"
	"time"

	"ass2/utils"
)

// RunSR implements Selective Repeat ARQ with window size N and optional NAK.
//
// Workflow:
//  1. Reader Goroutine: listens on TCP socket.
//     - TypeACK: sends seqNum to ackCh.
//     - TypeNAK: sends seqNum to nakCh for immediate fast retransmit.
//     - TypeEOF: terminates on EOF confirmation.
//  2. Transmit Loop:
//     - Transmits up to N frames ahead of base.
//     - Starts a dedicated timer (`time.AfterFunc`) for each transmitted frame.
//  3. Event Handling (`select` multiplexing):
//     - ACK received: marks frame acknowledged, stops its timer, updates RTT,
//       and slides the base forward over contiguous ACKed frames.
//     - NAK received: retransmits that specific frame immediately and resets
//       its timer (no need to wait for timer expiry).
//     - Per-frame Timer fired: pushes frame index to retxCh; sender backs off
//       RTO, retransmits that frame, and restarts its timer.
//  4. Once all frames are confirmed, sends EOF sentinel until acknowledged.
func (s *Sender) RunSR(n int, nakEnabled bool) {
	fmt.Printf("[SR] Starting transmission (window=%d, NAK=%v)...\n", n, nakEnabled)
	total := len(s.payloads)

	base, next := 0, 0
	acked := make(map[int]bool)
	sentAt := make(map[int]time.Time)
	var wmu sync.Mutex

	// Per-frame timers: when a timer fires it sends the frame index to retxCh
	timers := make(map[int]*time.Timer)
	retxCh := make(chan int, n*4) // frames to retransmit

	// ACK / NAK channels from the reader goroutine
	ackCh := make(chan uint8, n*4)
	nakCh := make(chan uint8, n*4)
	errCh := make(chan error, 1)

	// startFrameTimer creates/resets the per-frame timer for absolute index i.
	startFrameTimer := func(i int) {
		if t, ok := timers[i]; ok {
			t.Stop()
		}
		rto := s.rto.Get()
		timers[i] = time.AfterFunc(rto, func() {
			retxCh <- i
		})
	}

	// Goroutine: read ACK/NAK frames from TCP continuously
	go func() {
		for {
			raw, err := utils.ReadACKFrame(s.conn)
			if err != nil {
				errCh <- err
				return
			}
			f, err := utils.Deserialize(raw, s.cfg.FCSAlgo)
			if err != nil || f == nil {
				continue
			}
			switch f.Header.FrameType {
			case utils.TypeACK:
				ackCh <- f.Header.SeqNum
			case utils.TypeNAK:
				nakCh <- f.Header.SeqNum
			}
		}
	}()

	for {
		wmu.Lock()
		done := base >= total
		wmu.Unlock()
		if done {
			break
		}

		// Fill the window
		wmu.Lock()
		for next < base+n && next < total {
			seqNum := uint8(next % SeqModulus)
			frame := s.Framing(s.payloads[next], seqNum, utils.TypeData)
			now := time.Now()
			dropped, err := s.Send(frame)
			if err != nil {
				wmu.Unlock()
				fmt.Printf("[SR] Send error: %v\n", err)
				return
			}
			if !dropped {
				sentAt[next] = now
			}
			startFrameTimer(next)
			next++
		}
		wmu.Unlock()

		select {
		case ackSeq := <-ackCh:
			wmu.Lock()
			// Find absolute index for this seq num within [base, next)
			for i := base; i < next; i++ {
				if uint8(i%SeqModulus) == ackSeq && !acked[i] {
					acked[i] = true
					if t, ok := timers[i]; ok {
						t.Stop()
						delete(timers, i)
					}
					if t, ok := sentAt[i]; ok {
						s.Timeout(t)
						s.logFrame(i, ackSeq, t, time.Now(), false)
						delete(sentAt, i)
					}
					fmt.Printf("[SR] ACK seq=%d (frame %d)\n", ackSeq, i)
					break
				}
			}
			// Slide base forward over consecutive ACKed frames
			for base < next && acked[base] {
				delete(acked, base)
				base++
			}
			fmt.Printf("[SR] Window base=%d, next=%d, RTO=%.0fms\n",
				base, next, float64(s.rto.Get().Milliseconds()))
			wmu.Unlock()

		case nakSeq := <-nakCh:
			// Receiver detected corruption and requests immediate retransmit
			wmu.Lock()
			for i := base; i < next; i++ {
				if uint8(i%SeqModulus) == nakSeq && !acked[i] {
					fmt.Printf("[SR] NAK seq=%d — immediate retransmit of frame %d\n", nakSeq, i)
					seqNum := uint8(i % SeqModulus)
					frame := s.Framing(s.payloads[i], seqNum, utils.TypeData)
					now := time.Now()
					dropped, _ := s.Send(frame)
					if !dropped {
						sentAt[i] = now
					}
					startFrameTimer(i) // reset this frame's timer
					s.mu.Lock()
					s.retransmits++
					s.mu.Unlock()
					break
				}
			}
			wmu.Unlock()

		case retxIdx := <-retxCh:
			// Per-frame timer fired — retransmit only that frame
			wmu.Lock()
			if retxIdx >= base && retxIdx < next && !acked[retxIdx] {
				seqNum := uint8(retxIdx % SeqModulus)
				fmt.Printf("[SR] Timer expired for frame %d (seq %d) — retransmitting\n", retxIdx, seqNum)
				s.rto.Backoff()
				frame := s.Framing(s.payloads[retxIdx], seqNum, utils.TypeData)
				now := time.Now()
				dropped, _ := s.Send(frame)
				if !dropped {
					sentAt[retxIdx] = now
				}
				startFrameTimer(retxIdx) // restart timer for this frame
				s.mu.Lock()
				s.retransmits++
				s.mu.Unlock()
			}
			wmu.Unlock()

		case err := <-errCh:
			if err != nil {
				fmt.Printf("[SR] Connection error: %v\n", err)
			}
			return
		}
	}

	// Clean up lingering timers
	wmu.Lock()
	for _, t := range timers {
		t.Stop()
	}
	wmu.Unlock()

	// Unblock and terminate the background reader goroutine
	s.conn.SetDeadline(time.Now())
	<-errCh
	s.conn.SetDeadline(time.Time{})
}
