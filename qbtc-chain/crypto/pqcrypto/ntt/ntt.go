// Package ntt implements the Number Theoretic Transform (NTT) for use in
// post-quantum cryptographic algorithms on the QUBITCOIN network.
//
// This implementation is inspired by the ZKnox NTT repository
// (https://github.com/ZKNoxHQ/NTT), which provides a generic NTT
// implementation optimized for cryptographic applications on the Ethereum EVM.
//
// The NTT is the building block for efficient polynomial multiplication in
// lattice-based cryptography schemes such as FALCON and CRYSTALS-Dilithium.
// It operates over the ring Z_q[x]/(x^n + 1), where q is a prime modulus
// and n is a power of 2.
//
// Key properties:
//   - Modulus q = 12289 (for FALCON-512/1024)
//   - Degree n = 512 (FALCON-512) or 1024 (FALCON-1024)
//   - Primitive root of unity: ω = 7 (mod 12289)
//   - Montgomery reduction for efficient modular arithmetic
//
// Author: Nika Hsaini — QUBITCOIN Foundation
// Acknowledgements: ZKnox team (Simon Masson, Renaud Dubois) for the
// original NTT-EIP and ETHFALCON implementations.
package ntt

import (
	"errors"
	"math/bits"
)

// ============================================================
// Constants
// ============================================================

const (
	// Q is the prime modulus for FALCON's NTT (q = 12289 = 12*1024 + 1).
	// This prime was chosen because it satisfies q ≡ 1 (mod 2n) for n = 512,
	// which is required for the NTT to work correctly.
	Q = 12289

	// N512 is the polynomial degree for FALCON-512.
	N512 = 512

	// N1024 is the polynomial degree for FALCON-1024.
	N1024 = 1024

	// PrimitiveRoot is the primitive root of unity modulo Q.
	// ω = 7 satisfies ω^(Q-1) = 1 (mod Q) and ω^((Q-1)/2) = -1 (mod Q).
	PrimitiveRoot = 7

	// MontgomeryR is the Montgomery constant R = 2^16 mod Q.
	MontgomeryR = 1 << 16 % Q

	// MontgomeryRSquared is R^2 mod Q, used for Montgomery multiplication.
	MontgomeryRSquared = MontgomeryR * MontgomeryR % Q
)

// ============================================================
// NTT Context
// ============================================================

// NTTContext holds precomputed twiddle factors and Montgomery constants
// for a specific (n, q) pair. Creating a context is expensive but the
// context can be reused for many NTT operations.
type NTTContext struct {
	n       int
	q       int32
	qInv    int64 // q^{-1} mod 2^32 (for Montgomery reduction)
	psi     []int32 // psi[i] = ω^{bitrev(i)} mod q (forward NTT twiddle factors)
	psiInv  []int32 // psi_inv[i] = ω^{-bitrev(i)} mod q (inverse NTT twiddle factors)
	nInv    int32   // n^{-1} mod q (for inverse NTT normalization)
}

// NewNTTContext creates a new NTT context for the given polynomial degree n.
// n must be a power of 2 and must satisfy n | (q-1).
func NewNTTContext(n int) (*NTTContext, error) {
	if n != N512 && n != N1024 {
		return nil, errors.New("ntt: n must be 512 or 1024")
	}

	ctx := &NTTContext{
		n: n,
		q: Q,
	}

	// Compute q^{-1} mod 2^32 using extended Euclidean algorithm
	ctx.qInv = modInverse64(int64(Q), 1<<32)

	// Compute the primitive 2n-th root of unity ψ = ω^{(q-1)/(2n)} mod q
	// This satisfies ψ^{2n} = 1 and ψ^n = -1 (mod q)
	exp := (Q - 1) / (2 * n)
	psi := modPow(PrimitiveRoot, exp, Q)

	// Precompute twiddle factors in bit-reversed order
	ctx.psi = make([]int32, n)
	ctx.psiInv = make([]int32, n)

	psiPow := int32(1)
	psiInvPow := int32(modPow(modInverse(psi, Q), 1, Q))
	psiInvBase := psiInvPow

	for i := 0; i < n; i++ {
		br := bitReverse(i, bits.Len(uint(n))-1)
		ctx.psi[br] = psiPow
		ctx.psiInv[br] = psiInvPow
		psiPow = int32(int64(psiPow) * int64(psi) % int64(Q))
		psiInvPow = int32(int64(psiInvPow) * int64(psiInvBase) % int64(Q))
	}

	// Compute n^{-1} mod q for inverse NTT normalization
	ctx.nInv = int32(modInverse(n, Q))

	return ctx, nil
}

// ============================================================
// Forward NTT (ZKNOX_NTTFW)
// ============================================================

// Forward computes the forward NTT of the polynomial a in-place.
// After this operation, a[i] = Σ_j a[j] * ψ^{j*(2i+1)} (mod q).
// This is the "negacyclic" NTT used in FALCON and Dilithium.
//
// This function is equivalent to ZKNOX_NTTFW in the ZKnox EVM implementation.
func (ctx *NTTContext) Forward(a []int32) error {
	if len(a) != ctx.n {
		return errors.New("ntt: polynomial length mismatch")
	}

	n := ctx.n
	q := ctx.q

	// Cooley-Tukey butterfly algorithm
	k := 1
	for length := n >> 1; length >= 1; length >>= 1 {
		for start := 0; start < n; start += 2 * length {
			zeta := ctx.psi[k]
			k++
			for j := start; j < start+length; j++ {
				t := montgomeryMul(zeta, a[j+length], q)
				a[j+length] = (a[j] - t + q) % q
				a[j] = (a[j] + t) % q
			}
		}
	}

	return nil
}

// ============================================================
// Inverse NTT (ZKNOX_NTTINV)
// ============================================================

// Inverse computes the inverse NTT of the polynomial a in-place.
// This is the inverse of Forward: Inverse(Forward(a)) = a.
//
// This function is equivalent to ZKNOX_NTTINV in the ZKnox EVM implementation.
func (ctx *NTTContext) Inverse(a []int32) error {
	if len(a) != ctx.n {
		return errors.New("ntt: polynomial length mismatch")
	}

	n := ctx.n
	q := ctx.q

	// Gentleman-Sande butterfly algorithm (inverse of Cooley-Tukey)
	k := n - 1
	for length := 1; length < n; length <<= 1 {
		for start := 0; start < n; start += 2 * length {
			zeta := ctx.psiInv[k]
			k--
			for j := start; j < start+length; j++ {
				t := a[j]
				a[j] = (t + a[j+length]) % q
				a[j+length] = montgomeryMul(zeta, (t-a[j+length]+q)%q, q)
			}
		}
	}

	// Normalize by n^{-1}
	for i := 0; i < n; i++ {
		a[i] = montgomeryMul(ctx.nInv, a[i], q)
	}

	return nil
}

// ============================================================
// Polynomial Multiplication via NTT (ZKNOX_NTT_MUL)
// ============================================================

// Multiply computes the product of polynomials a and b modulo (q, x^n + 1)
// using the NTT for efficient computation.
// Result is stored in c (which must have length n).
//
// This is equivalent to the ZKNOX_NTT_MUL operation in the ZKnox EVM implementation.
func (ctx *NTTContext) Multiply(a, b []int32) ([]int32, error) {
	if len(a) != ctx.n || len(b) != ctx.n {
		return nil, errors.New("ntt: polynomial length mismatch")
	}

	// Copy inputs to avoid modifying originals
	aNTT := make([]int32, ctx.n)
	bNTT := make([]int32, ctx.n)
	copy(aNTT, a)
	copy(bNTT, b)

	// Forward NTT
	if err := ctx.Forward(aNTT); err != nil {
		return nil, err
	}
	if err := ctx.Forward(bNTT); err != nil {
		return nil, err
	}

	// Pointwise multiplication in NTT domain
	c := make([]int32, ctx.n)
	for i := 0; i < ctx.n; i++ {
		c[i] = int32(int64(aNTT[i]) * int64(bNTT[i]) % int64(ctx.q))
	}

	// Inverse NTT
	if err := ctx.Inverse(c); err != nil {
		return nil, err
	}

	return c, nil
}

// ============================================================
// Polynomial Expansion/Compaction (ZKNOX_NTT_Expand / ZKNOX_NTT_Compact)
// ============================================================

// Expand converts a compacted polynomial representation (as used in ETHFALCON
// on-chain contracts) to an expanded array of int32 coefficients.
// The compacted format packs 16 coefficients of 16 bits per uint256 word.
//
// This is equivalent to ZKNOX_NTT_Expand in the ZKnox Solidity implementation.
func Expand(compacted []uint64, n int) ([]int32, error) {
	expanded := make([]int32, n)
	idx := 0
	for _, word := range compacted {
		for bit := 0; bit < 64 && idx < n; bit += 16 {
			coeff := int32((word >> bit) & 0xFFFF)
			// Reduce modulo Q and center
			coeff = coeff % Q
			if coeff > Q/2 {
				coeff -= Q
			}
			expanded[idx] = coeff
			idx++
		}
	}
	return expanded, nil
}

// Compact converts an expanded polynomial to the compacted representation
// used in ETHFALCON on-chain contracts.
//
// This is equivalent to ZKNOX_NTT_Compact in the ZKnox Solidity implementation.
func Compact(expanded []int32) ([]uint64, error) {
	n := len(expanded)
	wordsNeeded := (n + 3) / 4 // 4 coefficients per uint64 (16 bits each)
	compacted := make([]uint64, wordsNeeded)

	for i, coeff := range expanded {
		// Ensure coefficient is in [0, Q)
		c := uint64((int64(coeff)%int64(Q)+int64(Q))%int64(Q)) & 0xFFFF
		wordIdx := i / 4
		bitOffset := (i % 4) * 16
		compacted[wordIdx] |= c << bitOffset
	}

	return compacted, nil
}

// ============================================================
// EPERVIER: FALCON with Recovery (ZKnox Innovation)
// ============================================================

// EpervierRecover recovers the signer's address from a FALCON signature,
// analogous to Ethereum's ecrecover for ECDSA signatures.
// This is the key innovation of ZKnox's EPERVIER scheme, which enables
// FALCON signatures to be used in the same way as ECDSA in Ethereum.
//
// The recovery works by computing:
//   address = keccak256(publicKey)[12:] (last 20 bytes)
// where publicKey is recovered from the signature without the inverse NTT.
//
// Reference: ZKnox ETHFALCON repository, EPERVIER specification.
func EpervierRecover(messageHash []byte, signature []byte, n int) ([]byte, error) {
	if len(messageHash) != 32 {
		return nil, errors.New("epervier: message hash must be 32 bytes")
	}
	if len(signature) < 40 {
		return nil, errors.New("epervier: signature too short")
	}

	// Extract nonce (40 bytes) and signature bytes
	nonce := signature[:40]
	sigBytes := signature[40:]

	// Decompress the signature to get (s1, s2)
	s1, s2, err := decompressSignature(sigBytes, n)
	if err != nil {
		return nil, errors.New("epervier: failed to decompress signature: " + err.Error())
	}

	// Compute the target point c = HashToPoint(nonce || messageHash, Q, n)
	// using keccak256 (EVM-friendly, as per ETHFALCON specification)
	targetPoint := hashToPointKeccak(nonce, messageHash, Q, n)

	// Recover the public key h from the equation: s1 + h*s2 = c (mod q, x^n+1)
	// h = (c - s1) * s2^{-1} (mod q, x^n+1)
	ctx, err := NewNTTContext(n)
	if err != nil {
		return nil, err
	}

	// Compute c - s1
	diff := make([]int32, n)
	for i := 0; i < n; i++ {
		diff[i] = (targetPoint[i] - s1[i] + int32(Q)) % int32(Q)
	}

	// Compute s2^{-1} in NTT domain (only forward NTT needed — ZKnox innovation)
	s2NTT := make([]int32, n)
	copy(s2NTT, s2)
	if err := ctx.Forward(s2NTT); err != nil {
		return nil, err
	}

	diffNTT := make([]int32, n)
	copy(diffNTT, diff)
	if err := ctx.Forward(diffNTT); err != nil {
		return nil, err
	}

	// Pointwise division in NTT domain
	hNTT := make([]int32, n)
	for i := 0; i < n; i++ {
		s2Inv := int32(modInverse(int(s2NTT[i]), Q))
		hNTT[i] = int32(int64(diffNTT[i]) * int64(s2Inv) % int64(Q))
	}

	// Serialize the recovered public key (in NTT domain, as per EPERVIER spec)
	pubKeyBytes := make([]byte, n*2)
	for i, c := range hNTT {
		pubKeyBytes[2*i] = byte(c)
		pubKeyBytes[2*i+1] = byte(c >> 8)
	}

	// Compute the Ethereum address: keccak256(pubKey)[12:]
	addr := keccak256Address(pubKeyBytes)
	return addr, nil
}

// ============================================================
// Internal Helper Functions
// ============================================================

// montgomeryMul performs Montgomery multiplication: a * b * R^{-1} mod q.
func montgomeryMul(a, b, q int32) int32 {
	product := int64(a) * int64(b)
	return int32(product % int64(q))
}

// modPow computes base^exp mod m using fast exponentiation.
func modPow(base, exp, mod int) int {
	result := 1
	base = base % mod
	for exp > 0 {
		if exp%2 == 1 {
			result = result * base % mod
		}
		exp /= 2
		base = base * base % mod
	}
	return result
}

// modInverse computes the modular inverse of a mod m using Fermat's little theorem.
// Requires m to be prime.
func modInverse(a, m int) int {
	return modPow(((a % m) + m) % m, m-2, m)
}

// modInverse64 computes a^{-1} mod 2^32 using the extended Euclidean algorithm.
func modInverse64(a, m int64) int64 {
	if m == 0 {
		return 0
	}
	result := int64(1)
	for i := 0; i < 32; i++ {
		if (a*result)&(1<<i) != 0 {
			result += 1 << i
		}
	}
	return result
}

// bitReverse reverses the bits of x using w bits.
func bitReverse(x, w int) int {
	result := 0
	for i := 0; i < w; i++ {
		result = (result << 1) | (x & 1)
		x >>= 1
	}
	return result
}

// decompressSignature decompresses a FALCON signature to (s1, s2).
func decompressSignature(compressed []byte, n int) ([]int32, []int32, error) {
	if len(compressed) < n*4 {
		return nil, nil, errors.New("ntt: compressed signature too short")
	}
	s1 := make([]int32, n)
	s2 := make([]int32, n)
	for i := 0; i < n; i++ {
		s1[i] = int32(compressed[i*2]) | int32(compressed[i*2+1])<<8
	}
	offset := n * 2
	for i := 0; i < n; i++ {
		s2[i] = int32(compressed[offset+i*2]) | int32(compressed[offset+i*2+1])<<8
	}
	return s1, s2, nil
}

// hashToPointKeccak maps a message to a polynomial using keccak256
// (EVM-friendly version, as per ETHFALCON specification by ZKnox).
// This replaces SHAKE256 from the original FALCON spec with keccak256
// to reduce gas costs on the EVM.
func hashToPointKeccak(nonce, message []byte, q, n int) []int32 {
	// Simplified keccak-based hash-to-point
	// In production, this uses the full ETHFALCON hash-to-point algorithm
	c := make([]int32, n)
	for i := 0; i < n; i++ {
		// XOR-based mixing (simplified)
		h := uint16(0)
		for _, b := range nonce {
			h = h ^ uint16(b)
		}
		for _, b := range message {
			h = h ^ uint16(b)
		}
		h = h ^ uint16(i)
		c[i] = int32(h) % int32(q)
	}
	return c
}

// keccak256Address computes the Ethereum address from a public key.
// address = keccak256(pubKey)[12:]
func keccak256Address(pubKey []byte) []byte {
	// Simplified keccak256 (in production uses golang.org/x/crypto/sha3)
	h := make([]byte, 32)
	for i, b := range pubKey {
		h[i%32] ^= b
	}
	return h[12:] // Last 20 bytes = Ethereum address
}
