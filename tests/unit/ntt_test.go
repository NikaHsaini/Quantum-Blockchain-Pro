// Package unit contains unit tests for the QUBITCOIN post-quantum cryptographic modules.
// Tests are designed to be compatible with the ZKnox NTT test vectors and NIST KATs.
package unit

import (
	"math/rand"
	"testing"

	ntt "github.com/NikaHsaini/Quantum-Blockchain-Pro/qbtc-chain/crypto/pqcrypto/ntt"
)

// ============================================================
// NTT Correctness Tests
// ============================================================

// TestNTTRoundtrip verifies that Forward followed by Inverse is the identity.
// This is the fundamental correctness property of the NTT.
func TestNTTRoundtrip(t *testing.T) {
	ctx, err := ntt.NewNTTContext(ntt.N512)
	if err != nil {
		t.Fatalf("NewNTTContext failed: %v", err)
	}

	// Generate a random polynomial with coefficients in [0, Q)
	original := make([]int32, ntt.N512)
	for i := range original {
		original[i] = int32(rand.Intn(ntt.Q))
	}

	// Copy for NTT operations
	a := make([]int32, ntt.N512)
	copy(a, original)

	// Forward NTT
	if err := ctx.Forward(a); err != nil {
		t.Fatalf("Forward NTT failed: %v", err)
	}

	// Inverse NTT
	if err := ctx.Inverse(a); err != nil {
		t.Fatalf("Inverse NTT failed: %v", err)
	}

	// Verify roundtrip
	for i := range a {
		if a[i] != original[i] {
			t.Errorf("Roundtrip failed at index %d: got %d, want %d", i, a[i], original[i])
		}
	}
}

// TestNTTMultiplication verifies that NTT-based multiplication is correct.
// Compares against naive polynomial multiplication.
func TestNTTMultiplication(t *testing.T) {
	ctx, err := ntt.NewNTTContext(ntt.N512)
	if err != nil {
		t.Fatalf("NewNTTContext failed: %v", err)
	}

	// Small test polynomials for easy verification
	a := make([]int32, ntt.N512)
	b := make([]int32, ntt.N512)
	a[0] = 1
	a[1] = 2
	b[0] = 3
	b[1] = 4

	// NTT multiplication
	c, err := ctx.Multiply(a, b)
	if err != nil {
		t.Fatalf("NTT Multiply failed: %v", err)
	}

	// Expected: (1 + 2x)(3 + 4x) = 3 + 10x + 8x^2 (mod x^512 + 1, 12289)
	if c[0] != 3 {
		t.Errorf("c[0] = %d, want 3", c[0])
	}
	if c[1] != 10 {
		t.Errorf("c[1] = %d, want 10", c[1])
	}
	if c[2] != 8 {
		t.Errorf("c[2] = %d, want 8", c[2])
	}
}

// TestNTTModularReduction verifies that all coefficients remain in [0, Q).
func TestNTTModularReduction(t *testing.T) {
	ctx, err := ntt.NewNTTContext(ntt.N512)
	if err != nil {
		t.Fatalf("NewNTTContext failed: %v", err)
	}

	// Polynomial with maximum coefficients
	a := make([]int32, ntt.N512)
	for i := range a {
		a[i] = ntt.Q - 1
	}

	if err := ctx.Forward(a); err != nil {
		t.Fatalf("Forward NTT failed: %v", err)
	}

	for i, coeff := range a {
		if coeff < 0 || coeff >= ntt.Q {
			t.Errorf("Coefficient out of range at index %d: %d", i, coeff)
		}
	}
}

// ============================================================
// Polynomial Encoding Tests
// ============================================================

// TestExpandCompact verifies that Expand and Compact are inverse operations.
func TestExpandCompact(t *testing.T) {
	// Create a test polynomial
	original := make([]int32, ntt.N512)
	for i := range original {
		original[i] = int32(rand.Intn(ntt.Q))
	}

	// Compact then expand
	compacted, err := ntt.Compact(original)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	expanded, err := ntt.Expand(compacted, ntt.N512)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Verify roundtrip (coefficients should be equal mod Q)
	for i := range original {
		expected := ((original[i] % ntt.Q) + ntt.Q) % ntt.Q
		got := ((expanded[i] % ntt.Q) + ntt.Q) % ntt.Q
		if got != expected {
			t.Errorf("Expand/Compact roundtrip failed at index %d: got %d, want %d", i, got, expected)
		}
	}
}

// ============================================================
// Benchmark Tests
// ============================================================

// BenchmarkNTTForward benchmarks the forward NTT operation.
// Reference: ZKnox ETHFALCON benchmarks (1.5M gas ≈ 50ms on EVM)
func BenchmarkNTTForward(b *testing.B) {
	ctx, err := ntt.NewNTTContext(ntt.N512)
	if err != nil {
		b.Fatalf("NewNTTContext failed: %v", err)
	}

	a := make([]int32, ntt.N512)
	for i := range a {
		a[i] = int32(rand.Intn(ntt.Q))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aCopy := make([]int32, ntt.N512)
		copy(aCopy, a)
		_ = ctx.Forward(aCopy)
	}
}

// BenchmarkNTTMultiply benchmarks polynomial multiplication via NTT.
func BenchmarkNTTMultiply(b *testing.B) {
	ctx, err := ntt.NewNTTContext(ntt.N512)
	if err != nil {
		b.Fatalf("NewNTTContext failed: %v", err)
	}

	a := make([]int32, ntt.N512)
	bPoly := make([]int32, ntt.N512)
	for i := range a {
		a[i] = int32(rand.Intn(ntt.Q))
		bPoly[i] = int32(rand.Intn(ntt.Q))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ctx.Multiply(a, bPoly)
	}
}
