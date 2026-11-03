package utils

import (
	"fmt"
	"math/rand/v2"
)

// InjectRandomBitError flips numErrors randomly selected bits in the given byte slice.
func InjectRandomBitError(data []byte, numErrors int) {
	if len(data) == 0 {
		return
	}
	for i := 0; i < numErrors; i++ {
		byteIdx := rand.IntN(len(data))
		bitIdx := rand.IntN(8)
		data[byteIdx] ^= (1 << bitIdx)
	}
}

// InjectSingleBitError flips exactly 1 random bit in data.
func InjectSingleBitError(data []byte) {
	InjectRandomBitError(data, 1)
}

// InjectDoubleBitError flips exactly 2 random bits in data.
func InjectDoubleBitError(data []byte) {
	InjectRandomBitError(data, 2)
}

// InjectOddBitError flips an odd number of random bits (e.g. 1, 3, 5, or 7 bits).
func InjectOddBitError(data []byte) {
	numErrorBits := rand.IntN(4)*2 + 1
	InjectRandomBitError(data, numErrorBits)
}

// InjectBurstError simulates a contiguous burst error of burstLength bits,
// where each bit in the burst has flipProb probability of being inverted.
func InjectBurstError(data []byte, burstLength int, flipProb float64) {
	totalBits := len(data) * 8
	if totalBits <= 0 || burstLength > totalBits {
		fmt.Println("Error applying burst")
		return
	}
	startBit := rand.IntN(totalBits - burstLength + 1)
	for i := 0; i < burstLength; i++ {
		if rand.Float64() < flipProb {
			currBit := startBit + i
			byteIdx := currBit / 8
			bitIdx := currBit % 8
			data[byteIdx] ^= (1 << bitIdx)
		}
	}
}
