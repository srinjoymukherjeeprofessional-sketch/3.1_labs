package common

import (
	"testing"
)

func TestWalshOrthogonality(t *testing.T) {
	for _, m := range []int{2, 4, 8, 16} {
		matrix, err := GenerateWalshMatrix(m)
		if err != nil {
			t.Fatalf("Failed to generate Walsh matrix for M=%d: %v", m, err)
		}
		if !VerifyOrthogonality(matrix) {
			t.Fatalf("Walsh matrix for M=%d failed orthogonality check", m)
		}
	}
}

func TestCDMASuperpositionAndReconstruction(t *testing.T) {
	// Simulate 4 stations with M = 4
	matrix, err := GenerateWalshMatrix(4)
	if err != nil {
		t.Fatal(err)
	}

	// Station 0 sends bit 1 (+1)
	// Station 1 sends bit 0 (-1)
	// Station 2 is silent (0)
	// Station 3 sends bit 1 (+1)
	bipolarVals := []int8{1, -1, 0, 1}
	chips := make([][]int8, 4)
	for i := 0; i < 4; i++ {
		chips[i] = EncodeBit(bipolarVals[i], matrix[i])
	}

	// Channel combines them:
	composite := Superposition(chips, 4)

	// Reconstruct at all 4 stations:
	for i := 0; i < 4; i++ {
		decoded, _ := DecodeChip(composite, matrix[i])
		if decoded != bipolarVals[i] {
			t.Errorf("Station %d: expected %d, got %d", i, bipolarVals[i], decoded)
		}
	}
}
