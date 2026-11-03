package utils

import (
	"math/rand/v2"
	"time"
)

// Channel emulates an unreliable network medium that can drop, delay, or corrupt frames.
type Channel struct {
	LossProb      float64       // Probability of completely dropping a packet (0.0 to 1.0)
	DelayProb     float64       // Probability of introducing artificial transmission delay
	DelayDuration time.Duration // Duration of delay when triggered
	CorruptProb   float64       // Probability of injecting bit errors into the frame
}

// NewChannel initializes a Channel with the specified impairment probabilities.
func NewChannel(lossProb, delayProb float64, delay time.Duration, corruptProb float64) *Channel {
	return &Channel{
		LossProb:      lossProb,
		DelayProb:     delayProb,
		DelayDuration: delay,
		CorruptProb:   corruptProb,
	}
}

// Transmit passes data through the simulated channel.
// Returns:
// - delivered: byte slice (possibly corrupted or delayed, or nil if dropped)
// - dropped: true if the packet was lost
// - delayed: true if the packet experienced delay
func (c *Channel) Transmit(data []byte) (delivered []byte, dropped bool, delayed bool) {
	if rand.Float64() < c.LossProb {
		return nil, true, false
	}
	if rand.Float64() < c.DelayProb {
		time.Sleep(c.DelayDuration)
		delayed = true
	}

	delivered = append([]byte(nil), data...)
	if rand.Float64() < c.CorruptProb {
		InjectSingleBitError(delivered)
	}
	return delivered, false, delayed
}
