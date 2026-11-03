package station

import (
	"ass3/common"
	"context"
	"fmt"
	"time"
)

// runCSMACD executes IEEE 802.3 CSMA/CD with Binary Exponential Backoff (BEB):
// 1. For each frame, set attempt counter k = 0.
// 2. Sense channel (1-Persistent) until IDLE.
// 3. Transmit frame (CmdTxStart -> Sleep Tfr -> CmdTxEnd).
// 4. Inspect server verdict:
//    - RespSuccess:
//        Reset k = 0, log success, wait IFG, advance to next frame.
//    - RespJam (Collision):
//        k = k + 1
//        If k > 16: abort frame (excessive collision), log drop, advance to next frame.
//        Else:
//            Compute backoff exponent: m = min(k, 10)
//            Draw random slots: r in [0, 2^m - 1]
//            Sleep r * SlotTime
//            Retransmit same frame (loop back to step 2).
func (s *Station) runCSMACD(ctx context.Context) error {
	pollingInterval := 500 * time.Microsecond

	for frameSeq := uint32(1); frameSeq <= uint32(s.TotalFrames); frameSeq++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frameStart := time.Now()
		attempt := 0

		// Retransmission loop for current frame
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Step 1: 1-Persistent sensing until channel is IDLE
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

			// Step 2: Transmit frame
			s.metrics.FramesAttempted++
			startTx := time.Now()
			isSuccess, err := s.TransmitFrame(frameSeq)
			if err != nil {
				return err
			}
			s.metrics.TotalTxTime += time.Since(startTx)

			// Step 3: Handle outcome
			if isSuccess {
				s.metrics.FramesSuccessful++
				if !s.Quiet {
					if attempt == 0 {
						fmt.Printf("[Station %2d] [CSMA/CD] Frame #%d: SUCCESS on 1st try\n", s.ID, frameSeq)
					} else {
						fmt.Printf("[Station %2d] [CSMA/CD] Frame #%d: SUCCESS after %d collision(s)\n", s.ID, frameSeq, attempt)
					}
				}
				s.metrics.TotalDelay += time.Since(frameStart)
				time.Sleep(s.IFG)
				break // Frame delivered! Advance to next frame
			}

			// Step 4: Collision detected!
			attempt++
			s.metrics.Collisions++

			if attempt > common.MaxBEBAttempts {
				// Abort frame after 16 consecutive collisions
				s.metrics.FramesDropped++
				if !s.Quiet {
					fmt.Printf("[Station %2d] [CSMA/CD] Frame #%d: DROPPED (Exceeded %d collisions)\n",
						s.ID, frameSeq, common.MaxBEBAttempts)
				}
				s.metrics.TotalDelay += time.Since(frameStart)
				time.Sleep(s.IFG)
				break // Abort frame, advance to next
			}

			// Step 5: Binary Exponential Backoff (BEB)
			exp := attempt
			if exp > common.MaxBEBExponent {
				exp = common.MaxBEBExponent
			}
			maxSlots := 1 << exp // 2^exp
			r := s.randInt(maxSlots)
			backoffDuration := time.Duration(r) * s.SlotTime

			if !s.Quiet {
				fmt.Printf("[Station %2d] [CSMA/CD] Frame #%d: COLLISION (attempt %d/16) -> Backing off %d slots (%v)\n",
					s.ID, frameSeq, attempt, r, backoffDuration)
			}

			s.metrics.TotalWaitTime += backoffDuration
			time.Sleep(backoffDuration)
			// Loop back and retransmit current frame
		}
	}
	return nil
}
