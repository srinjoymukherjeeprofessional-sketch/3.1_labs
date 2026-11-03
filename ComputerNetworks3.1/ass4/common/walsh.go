package common

import (
	"fmt"
)

// NextPowerOf2 returns the smallest power of 2 greater than or equal to n.
// For CDMA, the Walsh code length M must be a power of 2 with M >= n.
func NextPowerOf2(n int) int {
	if n <= 1 {
		return 2
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// GenerateWalshMatrix creates an M x M Hadamard/Walsh matrix using Sylvester's construction.
// Elements are +1 and -1.
// H_1 = [[1]]
// H_2k = [ [H_k,  H_k],
//          [H_k, -H_k] ]
func GenerateWalshMatrix(m int) ([][]int8, error) {
	if m < 1 || (m&(m-1)) != 0 {
		return nil, fmt.Errorf("walsh matrix size M must be a power of 2, got %d", m)
	}

	matrix := make([][]int8, m)
	for i := range matrix {
		matrix[i] = make([]int8, m)
	}

	// Base case: H_1 = [1]
	matrix[0][0] = 1

	// Iteratively double the matrix until size m
	for k := 1; k < m; k <<= 1 {
		for i := 0; i < k; i++ {
			for j := 0; j < k; j++ {
				val := matrix[i][j]
				matrix[i][j] = val         // Top-left: H_k
				matrix[i][j+k] = val       // Top-right: H_k
				matrix[i+k][j] = val       // Bottom-left: H_k
				matrix[i+k][j+k] = -val    // Bottom-right: -H_k
			}
		}
	}

	return matrix, nil
}

// BitToBipolar converts a binary bit (0 or 1) or silence indicator into bipolar value:
//   1 -> +1
//   0 -> -1
//  -1 or any other (silent/idle) -> 0
func BitToBipolar(bit int) int8 {
	switch bit {
	case 1:
		return 1
	case 0:
		return -1
	default:
		return 0 // Silent
	}
}

// BipolarToBit converts bipolar value back into bit representation:
//  +1 -> 1
//  -1 -> 0
//   0 -> -1 (Silent)
func BipolarToBit(bipolar int8) int {
	switch bipolar {
	case 1:
		return 1
	case -1:
		return 0
	default:
		return -1 // Silent
	}
}

// EncodeBit multiplies a station's bipolar data value with its unique Walsh code.
// Resulting chip vector has length M.
// If bipolar == 0 (silent), returns an all-zero chip vector.
func EncodeBit(bipolar int8, walshCode []int8) []int8 {
	chips := make([]int8, len(walshCode))
	for i, w := range walshCode {
		chips[i] = bipolar * w
	}
	return chips
}

// Superposition computes the linear algebraic sum of concurrent chip vectors
// across all transmitting stations.
// C[k] = SUM_{i=0..N-1} chips[i][k]
func Superposition(chipVectors [][]int8, m int) []int16 {
	composite := make([]int16, m)
	for _, vec := range chipVectors {
		for k := 0; k < m && k < len(vec); k++ {
			composite[k] += int16(vec[k])
		}
	}
	return composite
}

// DecodeChip computes the normalized inner product (dot product) of the composite
// channel signal with the station's Walsh code:
//   innerProduct = (composite . walshCode)
//   decision     = innerProduct / M
// Returns the reconstructed bipolar value (+1, -1, or 0) and the raw inner product sum.
func DecodeChip(composite []int16, walshCode []int8) (int8, int) {
	m := len(walshCode)
	if m == 0 || len(composite) != m {
		return 0, 0
	}

	var sum int
	for k := 0; k < m; k++ {
		sum += int(composite[k]) * int(walshCode[k])
	}

	// Normalized decision
	decision := sum / m
	if decision > 0 {
		return 1, sum
	} else if decision < 0 {
		return -1, sum
	}
	return 0, sum
}

// DotProduct calculates the inner product of two Walsh vectors of the same length.
func DotProduct(a, b []int8) int {
	if len(a) != len(b) {
		return 0
	}
	var sum int
	for i := range a {
		sum += int(a[i]) * int(b[i])
	}
	return sum
}

// VerifyOrthogonality checks if the given Walsh matrix is fully orthogonal.
// H[i] . H[j] == 0 for i != j, and H[i] . H[i] == M.
func VerifyOrthogonality(matrix [][]int8) bool {
	m := len(matrix)
	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			dot := DotProduct(matrix[i], matrix[j])
			if i == j {
				if dot != m {
					return false
				}
			} else {
				if dot != 0 {
					return false
				}
			}
		}
	}
	return true
}
