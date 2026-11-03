// sender/timeout.go
//
// ─── Adaptive Timeout & RTO Management ────────────────────────────────
//
//   1. First Measurement:
//      When the first ACK arrives with measured RTT m:
//          SRTT = m
//
//   2. Subsequent Measurements:
//      Average the previous SRTT with the latest measured RTT m:
//          SRTT = (SRTT + m) / 2
//
//   3. Retransmission Timeout (RTO):
//      RTO is set to twice the smoothed RTT to provide a generous safety margin:
//          RTO = 2 * SRTT
//      Clamped to [minRTO, maxRTO] (200ms to 30s) to prevent zero or runaway values.
//
//   4. Exponential Backoff (on timeout):
//      If a timeout occurs before receiving an ACK:
//          RTO = min(2 * RTO, maxRTO)
//      Doubling the timeout on successive failures prevents congesting an already lossy link.
package main

import (
	"math"
	"sync"
	"time"
)

// ─── RTO Limits ───────────────────────────────────────────────────────────────

const (
	initRTO = 1 * time.Second        // Initial RTO before any RTT measurement is taken
	minRTO  = 50 * time.Millisecond // Floor: minimum timeout duration
	maxRTO  = 3 * time.Second       // Ceiling: maximum timeout duration
)

// ─── RTOState Struct ──────────────────────────────────────────────────────────

// RTOState maintains the smoothed RTT and the current retransmission timeout.
type RTOState struct {
	mu       sync.Mutex
	srtt     time.Duration // Smoothed RTT: average of previous srtt and latest rtt
	rto      time.Duration // Current timeout duration (2 * srtt)
	measured bool          // false until the first valid RTT sample is collected
}

// newRTOState initializes an RTOState with the default initial RTO.
func newRTOState() *RTOState {
	return &RTOState{rto: initRTO}
}

// Update incorporates a new RTT measurement m:
//   - On first sample:  srtt = m
//   - Subsequent:       srtt = (srtt + m) / 2
//   - RTO:              rto  = 2 * srtt (clamped to [minRTO, maxRTO])
func (r *RTOState) Update(m time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.measured {
		r.srtt = m
		r.measured = true
	} else {
		// Average of previous SRTT and latest RTT
		r.srtt = (r.srtt + m) / 2
	}

	// Timeout is set to double the smoothed RTT
	r.rto = min(max(2*r.srtt, minRTO), maxRTO)
}

// Backoff doubles RTO on timeout (exponential backoff).
func (r *RTOState) Backoff() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rto = time.Duration(math.Min(float64(r.rto*2), float64(maxRTO)))
}

// Get returns the current RTO value in a thread-safe manner.
func (r *RTOState) Get() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rto
}

// ─── Sender Timeout Helpers ───────────────────────────────────────────────────

// Timeout records a successfully measured RTT and updates the RTO estimate.
// sentAt is the timestamp when the frame was originally transmitted.
func (s *Sender) Timeout(sentAt time.Time) {
	rtt := time.Since(sentAt)
	s.rto.Update(rtt)
}

// Timer starts a new one-shot timer set to the current RTO duration.
// The returned *time.Timer fires into its C channel upon expiry.
func (s *Sender) Timer() *time.Timer {
	return time.NewTimer(s.rto.Get())
}
