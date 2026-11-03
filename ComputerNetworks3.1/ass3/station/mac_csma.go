package station

import (
	"context"
	"fmt"
	"time"
)

// runNonPersistent executes Non-Persistent CSMA:
// 1. Sense channel.
// 2. If BUSY: wait random backoff (1 to 5 slots) and re-sense.
// 3. If IDLE: transmit immediately.
// 4. Fire-and-forget: receive result, update metrics, wait IFG, advance to next frame.
func (s *Station) runNonPersistent(ctx context.Context) error {
	for frameSeq := uint32(1); frameSeq <= uint32(s.TotalFrames); frameSeq++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frameStart := time.Now()

		// Contention loop for current frame
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			isIdle, err := s.SenseChannel()
			if err != nil {
				return err
			}

			if !isIdle {
				// Channel is BUSY: Backoff random slots [1, 5]
				backoffSlots := s.randInt(5) + 1
				backoffTime := time.Duration(backoffSlots) * s.SlotTime
				s.metrics.TotalWaitTime += backoffTime
				time.Sleep(backoffTime)
				continue
			}

			// Channel is IDLE: Transmit frame
			s.metrics.FramesAttempted++
			startTx := time.Now()
			isSuccess, err := s.TransmitFrame(frameSeq)
			if err != nil {
				return err
			}
			s.metrics.TotalTxTime += time.Since(startTx)

			if isSuccess {
				s.metrics.FramesSuccessful++
				if !s.Quiet {
					fmt.Printf("[Station %2d] [Non-P] Frame #%d: SUCCESS\n", s.ID, frameSeq)
				}
			} else {
				s.metrics.Collisions++
				s.metrics.FramesDropped++ // Pure CSMA drops collided frame (no MAC retransmissions)
				if !s.Quiet {
					fmt.Printf("[Station %2d] [Non-P] Frame #%d: COLLISION (Dropped)\n", s.ID, frameSeq)
				}
			}

			s.metrics.TotalDelay += time.Since(frameStart)
			time.Sleep(s.IFG)
			break // Advance to next frame
		}
	}
	return nil
}

// run1Persistent executes 1-Persistent CSMA:
// 1. Sense channel continuously (polling interval ~0.5ms) until IDLE.
// 2. Transmit immediately with probability 1.
// 3. Fire-and-forget: receive result, update metrics, wait IFG, advance to next frame.
func (s *Station) run1Persistent(ctx context.Context) error {
	pollingInterval := 500 * time.Microsecond

	for frameSeq := uint32(1); frameSeq <= uint32(s.TotalFrames); frameSeq++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frameStart := time.Now()

		// Wait until channel becomes IDLE
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			isIdle, err := s.SenseChannel()
			if err != nil {
				return err
			}

			if isIdle {
				break
			}

			s.metrics.TotalWaitTime += pollingInterval
			time.Sleep(pollingInterval)
		}

		// Transmit immediately
		s.metrics.FramesAttempted++
		startTx := time.Now()
		isSuccess, err := s.TransmitFrame(frameSeq)
		if err != nil {
			return err
		}
		s.metrics.TotalTxTime += time.Since(startTx)

		if isSuccess {
			s.metrics.FramesSuccessful++
			if !s.Quiet {
				fmt.Printf("[Station %2d] [1-P] Frame #%d: SUCCESS\n", s.ID, frameSeq)
			}
		} else {
			s.metrics.Collisions++
			s.metrics.FramesDropped++ // Pure CSMA drops collided frame
			if !s.Quiet {
				fmt.Printf("[Station %2d] [1-P] Frame #%d: COLLISION (Dropped)\n", s.ID, frameSeq)
			}
		}

		s.metrics.TotalDelay += time.Since(frameStart)
		time.Sleep(s.IFG)
	}
	return nil
}

// runPPersistent executes p-Persistent CSMA:
// 1. Sense channel continuously until IDLE.
// 2. When IDLE:
//    - With probability p: transmit immediately.
//    - With probability (1 - p): wait 1 SlotTime (2*Tp).
//      Re-sense: if still IDLE, repeat step 2. If BUSY, defer until IDLE again (step 1).
// 3. Fire-and-forget: receive result, update metrics, wait IFG, advance to next frame.
func (s *Station) runPPersistent(ctx context.Context) error {
	pollingInterval := 500 * time.Microsecond

	for frameSeq := uint32(1); frameSeq <= uint32(s.TotalFrames); frameSeq++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frameStart := time.Now()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Step 1: Wait until IDLE
			for {
				isIdle, err := s.SenseChannel()
				if err != nil {
					return err
				}
				if isIdle {
					break
				}
				s.metrics.TotalWaitTime += pollingInterval
				time.Sleep(pollingInterval)
			}

			// Step 2: Probability test
			u := s.randFloat()
			if u <= s.PVal {
				// Transmit with probability p
				s.metrics.FramesAttempted++
				startTx := time.Now()
				isSuccess, err := s.TransmitFrame(frameSeq)
				if err != nil {
					return err
				}
				s.metrics.TotalTxTime += time.Since(startTx)

				if isSuccess {
					s.metrics.FramesSuccessful++
					if !s.Quiet {
						fmt.Printf("[Station %2d] [p-P (p=%.2f)] Frame #%d: SUCCESS\n", s.ID, s.PVal, frameSeq)
					}
				} else {
					s.metrics.Collisions++
					s.metrics.FramesDropped++
					if !s.Quiet {
						fmt.Printf("[Station %2d] [p-P (p=%.2f)] Frame #%d: COLLISION (Dropped)\n", s.ID, s.PVal, frameSeq)
					}
				}

				s.metrics.TotalDelay += time.Since(frameStart)
				time.Sleep(s.IFG)
				break // Done with this frame, advance to next
			}

			// With probability (1 - p): Defer 1 SlotTime
			s.metrics.TotalWaitTime += s.SlotTime
			time.Sleep(s.SlotTime)

			// Re-sense after deferral
			isStillIdle, err := s.SenseChannel()
			if err != nil {
				return err
			}
			if !isStillIdle {
				// Channel became busy during deferral; go back to Step 1 (wait until idle again)
				continue
			}
			// If still idle, loop around to Step 2 (re-evaluate probability)
		}
	}
	return nil
}
