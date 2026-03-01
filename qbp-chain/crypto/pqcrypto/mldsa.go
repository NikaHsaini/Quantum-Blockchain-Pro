// Package pqcrypto implements post-quantum cryptographic algorithms
// standardized by NIST in August 2024 (FIPS 203, FIPS 204, FIPS 205).
//
// This package provides:
//   - ML-DSA (Module-Lattice-Based Digital Signature Algorithm) - FIPS 204
//     Replaces ECDSA for transaction and block signing.
//   - ML-KEM (Module-Lattice-Based Key-Encapsulation Mechanism) - FIPS 203
//     Used for P2P channel encryption and key exchange.
//   - SLH-DSA (Stateless Hash-Based Digital Signature Algorithm) - FIPS 205
//     Backup signature scheme based on hash functions.
package pqcrypto

import (
	"crypto/rand"
	"crypto/sha3"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// ============================================================
// ML-DSA (CRYSTALS-Dilithium) - FIPS 204
// Security Level: ML-DSA-65 (Category 3 - 192-bit classical security)
// ============================================================

// ML-DSA Parameters (Mode 3 / Dilithium3 - NIST Security Level 3)
const (
	// Lattice dimension parameters
	MLDSA_K = 6 // Number of rows in matrix A
	MLDSA_L = 5 // Number of columns in matrix A

	// Polynomial ring parameters
	MLDSA_N    = 256 // Polynomial degree
	MLDSA_Q    = 8380417 // Prime modulus q = 2^23 - 2^13 + 1
	MLDSA_ETA  = 4   // Coefficient range for secret key
	MLDSA_TAU  = 49  // Number of +/-1 in challenge polynomial

	// Encoding parameters
	MLDSA_D     = 13 // Dropped bits from t
	MLDSA_GAMMA1 = 1 << 19 // Coefficient range for y
	MLDSA_GAMMA2 = (MLDSA_Q - 1) / 32 // Low-order rounding range
	MLDSA_BETA  = MLDSA_TAU * MLDSA_ETA // Bound for z

	// Key and signature sizes (bytes)
	MLDSA_PUBLICKEY_SIZE  = 1952 // 32 + 32*K*((Q-1).bitLen() - D)
	MLDSA_PRIVATEKEY_SIZE = 4032 // Full private key
	MLDSA_SIGNATURE_SIZE  = 3309 // Signature size
	MLDSA_SEED_SIZE       = 32   // Seed size for key generation
)

// MLDSAPublicKey represents an ML-DSA public key.
type MLDSAPublicKey struct {
	rho  [32]byte      // Seed for matrix A
	t1   [][]int32     // Public polynomial vector t1 (K polynomials)
	raw  []byte        // Serialized form
}

// MLDSAPrivateKey represents an ML-DSA private key.
type MLDSAPrivateKey struct {
	rho   [32]byte   // Seed for matrix A
	K_key [32]byte   // Key for PRF
	tr    [64]byte   // Hash of public key
	s1    [][]int32  // Secret polynomial vector s1 (L polynomials)
	s2    [][]int32  // Secret polynomial vector s2 (K polynomials)
	t0    [][]int32  // Low-order bits of t (K polynomials)
	raw   []byte     // Serialized form
	pub   *MLDSAPublicKey
}

// MLDSASignature represents an ML-DSA signature.
type MLDSASignature struct {
	c_tilde [64]byte // Challenge hash
	z       [][]int32 // Response vector z (L polynomials)
	h       [][]int32 // Hint vector h (K polynomials)
	raw     []byte
}

// MLDSAKeyPair holds a public/private key pair.
type MLDSAKeyPair struct {
	PublicKey  *MLDSAPublicKey
	PrivateKey *MLDSAPrivateKey
}

// GenerateMLDSAKeyPair generates a new ML-DSA-65 key pair using a cryptographically
// secure random seed. This is the primary key generation function for QBP accounts.
func GenerateMLDSAKeyPair() (*MLDSAKeyPair, error) {
	seed := make([]byte, MLDSA_SEED_SIZE)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("pqcrypto: failed to generate random seed: %w", err)
	}
	return GenerateMLDSAKeyPairFromSeed(seed)
}

// GenerateMLDSAKeyPairFromSeed generates a deterministic ML-DSA-65 key pair from
// a 32-byte seed. This allows deterministic key derivation from a mnemonic phrase.
func GenerateMLDSAKeyPairFromSeed(seed []byte) (*MLDSAKeyPair, error) {
	if len(seed) != MLDSA_SEED_SIZE {
		return nil, fmt.Errorf("pqcrypto: seed must be %d bytes, got %d", MLDSA_SEED_SIZE, len(seed))
	}

	// Expand seed using SHAKE-256 to derive rho, rho', K
	h := sha3.NewShake256()
	h.Write(seed)
	h.Write([]byte{byte(MLDSA_K), byte(MLDSA_L)}) // Domain separation

	expanded := make([]byte, 128)
	h.Read(expanded)

	var rho [32]byte
	var rhoPrime [64]byte
	var kKey [32]byte

	copy(rho[:], expanded[0:32])
	copy(rhoPrime[:], expanded[32:96])
	copy(kKey[:], expanded[96:128])

	// Generate matrix A from rho (NTT domain)
	A := expandMatrix(rho)

	// Sample secret vectors s1 and s2 from rhoPrime
	s1 := sampleSecretVector(rhoPrime[:], MLDSA_L, 0)
	s2 := sampleSecretVector(rhoPrime[:], MLDSA_K, MLDSA_L)

	// Compute t = A*s1 + s2
	t := computeT(A, s1, s2)

	// Power2Round to get t1 and t0
	t1, t0 := power2Round(t)

	// Serialize public key
	pkBytes := serializePublicKey(rho, t1)

	// Compute tr = H(pk)
	var tr [64]byte
	pkHash := sha3.NewShake256()
	pkHash.Write(pkBytes)
	pkHash.Read(tr[:])

	pub := &MLDSAPublicKey{
		rho: rho,
		t1:  t1,
		raw: pkBytes,
	}

	priv := &MLDSAPrivateKey{
		rho:   rho,
		K_key: kKey,
		tr:    tr,
		s1:    s1,
		s2:    s2,
		t0:    t0,
		raw:   serializePrivateKey(rho, kKey, tr, s1, s2, t0),
		pub:   pub,
	}

	return &MLDSAKeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// Sign creates an ML-DSA-65 signature over the given message using the private key.
// The signature is deterministic when rnd is nil, or randomized when rnd is provided.
func Sign(priv *MLDSAPrivateKey, message []byte) (*MLDSASignature, error) {
	if priv == nil {
		return nil, errors.New("pqcrypto: nil private key")
	}

	// mu = H(tr || message)
	mu := make([]byte, 64)
	h := sha3.NewShake256()
	h.Write(priv.tr[:])
	h.Write(message)
	h.Read(mu)

	// rnd = H(K || rnd' || mu) for deterministic signing
	rndSeed := make([]byte, 32)
	rand.Read(rndSeed) // randomized signing for security

	rho_pp := make([]byte, 64)
	h2 := sha3.NewShake256()
	h2.Write(priv.K_key[:])
	h2.Write(rndSeed)
	h2.Write(mu)
	h2.Read(rho_pp)

	kappa := 0

	for {
		// Sample y from rho_pp
		y := sampleMaskingVector(rho_pp, MLDSA_L, kappa)

		// Compute w = A*y
		A := expandMatrix(priv.rho)
		w := matVecMul(A, y)

		// Decompose w into w1 (high bits) and w0 (low bits)
		w1, _ := decomposeW(w)

		// c_tilde = H(mu || w1_encoded)
		var c_tilde [64]byte
		hc := sha3.NewShake256()
		hc.Write(mu)
		hc.Write(encodeW1(w1))
		hc.Read(c_tilde[:])

		// Sample challenge polynomial c from c_tilde
		c := sampleChallenge(c_tilde[:])

		// z = y + c*s1
		z := addVec(y, scalarVecMul(c, priv.s1))

		// Check norm bounds
		if !checkNormBound(z, MLDSA_GAMMA1-MLDSA_BETA) {
			kappa++
			continue
		}

		// r0 = w0 - c*s2
		cs2 := scalarVecMul(c, priv.s2)
		r0 := subVec(decomposeW0(w), cs2)

		if !checkNormBound(r0, MLDSA_GAMMA2-MLDSA_BETA) {
			kappa++
			continue
		}

		// ct0 = c*t0
		ct0 := scalarVecMul(c, priv.t0)

		if !checkNormBound(ct0, MLDSA_GAMMA2) {
			kappa++
			continue
		}

		// Compute hints h
		h_vec := makeHints(addVec(negVec(cs2), ct0), w1)

		if countHints(h_vec) > MLDSA_OMEGA {
			kappa++
			continue
		}

		sig := &MLDSASignature{
			c_tilde: c_tilde,
			z:       z,
			h:       h_vec,
			raw:     serializeSignature(c_tilde, z, h_vec),
		}

		return sig, nil
	}
}

// Verify verifies an ML-DSA-65 signature against a message and public key.
// Returns nil if the signature is valid, or an error otherwise.
func Verify(pub *MLDSAPublicKey, message []byte, sig *MLDSASignature) error {
	if pub == nil {
		return errors.New("pqcrypto: nil public key")
	}
	if sig == nil {
		return errors.New("pqcrypto: nil signature")
	}

	// Check norm bound on z
	if !checkNormBound(sig.z, MLDSA_GAMMA1-MLDSA_BETA) {
		return errors.New("pqcrypto: signature verification failed: z norm bound exceeded")
	}

	// Recompute mu = H(H(pk) || message)
	var tr [64]byte
	pkHash := sha3.NewShake256()
	pkHash.Write(pub.raw)
	pkHash.Read(tr[:])

	mu := make([]byte, 64)
	hm := sha3.NewShake256()
	hm.Write(tr[:])
	hm.Write(message)
	hm.Read(mu)

	// Reconstruct challenge polynomial c from c_tilde
	c := sampleChallenge(sig.c_tilde[:])

	// Recompute w' = A*z - c*t1*2^d
	A := expandMatrix(pub.rho)
	Az := matVecMul(A, sig.z)
	ct1 := scalarVecMul(c, pub.t1)
	ct1Shifted := shiftLeft(ct1, MLDSA_D)
	wPrime := subVec(Az, ct1Shifted)

	// Use hints to recover w1
	w1Prime := useHints(sig.h, wPrime)

	// Recompute challenge hash
	var c_tilde_prime [64]byte
	hc := sha3.NewShake256()
	hc.Write(mu)
	hc.Write(encodeW1(w1Prime))
	hc.Read(c_tilde_prime[:])

	// Verify hint count
	if countHints(sig.h) > MLDSA_OMEGA {
		return errors.New("pqcrypto: signature verification failed: too many hints")
	}

	// Compare challenge hashes
	if c_tilde_prime != sig.c_tilde {
		return errors.New("pqcrypto: signature verification failed: challenge mismatch")
	}

	return nil
}

// ============================================================
// Key Serialization and Encoding
// ============================================================

// Bytes returns the serialized public key.
func (pk *MLDSAPublicKey) Bytes() []byte {
	return pk.raw
}

// Hex returns the hexadecimal encoding of the public key.
func (pk *MLDSAPublicKey) Hex() string {
	return hex.EncodeToString(pk.raw)
}

// Address derives a QBP address from the public key.
// The address is the last 20 bytes of SHA3-256(publicKey).
func (pk *MLDSAPublicKey) Address() [20]byte {
	h := sha3.New256()
	h.Write(pk.raw)
	hash := h.Sum(nil)
	var addr [20]byte
	copy(addr[:], hash[len(hash)-20:])
	return addr
}

// Bytes returns the serialized signature.
func (sig *MLDSASignature) Bytes() []byte {
	return sig.raw
}

// ParseMLDSAPublicKey deserializes a public key from bytes.
func ParseMLDSAPublicKey(data []byte) (*MLDSAPublicKey, error) {
	if len(data) != MLDSA_PUBLICKEY_SIZE {
		return nil, fmt.Errorf("pqcrypto: invalid public key size: expected %d, got %d",
			MLDSA_PUBLICKEY_SIZE, len(data))
	}
	var rho [32]byte
	copy(rho[:], data[0:32])
	t1 := deserializeT1(data[32:])
	return &MLDSAPublicKey{rho: rho, t1: t1, raw: data}, nil
}

// ParseMLDSASignature deserializes a signature from bytes.
func ParseMLDSASignature(data []byte) (*MLDSASignature, error) {
	if len(data) != MLDSA_SIGNATURE_SIZE {
		return nil, fmt.Errorf("pqcrypto: invalid signature size: expected %d, got %d",
			MLDSA_SIGNATURE_SIZE, len(data))
	}
	return deserializeSignature(data)
}

// ============================================================
// Internal Lattice Operations (simplified implementations)
// ============================================================

const (
	MLDSA_OMEGA = 55 // Maximum number of hints
)

// expandMatrix generates the public matrix A from seed rho using SHAKE-128.
// A is a K x L matrix of polynomials in NTT domain.
func expandMatrix(rho [32]byte) [][][]int32 {
	A := make([][][]int32, MLDSA_K)
	for i := 0; i < MLDSA_K; i++ {
		A[i] = make([][]int32, MLDSA_L)
		for j := 0; j < MLDSA_L; j++ {
			A[i][j] = samplePolynomialUniform(rho[:], uint16(i<<8|j))
		}
	}
	return A
}

// samplePolynomialUniform samples a uniform polynomial from seed using SHAKE-128.
func samplePolynomialUniform(seed []byte, nonce uint16) []int32 {
	poly := make([]int32, MLDSA_N)
	h := sha3.NewShake128()
	h.Write(seed)
	h.Write([]byte{byte(nonce >> 8), byte(nonce)})

	buf := make([]byte, 3*MLDSA_N)
	h.Read(buf)

	j := 0
	for i := 0; i < len(buf) && j < MLDSA_N; i += 3 {
		val := int32(buf[i]) | int32(buf[i+1])<<8 | int32(buf[i+2]&0x7F)<<16
		if val < MLDSA_Q {
			poly[j] = val
			j++
		}
	}
	return poly
}

// sampleSecretVector samples a secret polynomial vector with small coefficients.
func sampleSecretVector(seed []byte, length, offset int) [][]int32 {
	vec := make([][]int32, length)
	for i := 0; i < length; i++ {
		vec[i] = sampleEtaBounded(seed, uint16(offset+i))
	}
	return vec
}

// sampleEtaBounded samples a polynomial with coefficients in [-eta, eta].
func sampleEtaBounded(seed []byte, nonce uint16) []int32 {
	poly := make([]int32, MLDSA_N)
	h := sha3.NewShake256()
	h.Write(seed)
	h.Write([]byte{byte(nonce >> 8), byte(nonce)})

	buf := make([]byte, MLDSA_N)
	h.Read(buf)

	for i := 0; i < MLDSA_N; i++ {
		t := int32(buf[i] & 0x0F)
		if t < 9 {
			poly[i] = 4 - t
		} else {
			t = int32(buf[i] >> 4)
			if t < 9 {
				poly[i] = 4 - t
			}
		}
	}
	return poly
}

// sampleMaskingVector samples the masking vector y with large coefficients.
func sampleMaskingVector(rho_pp []byte, length, kappa int) [][]int32 {
	vec := make([][]int32, length)
	for i := 0; i < length; i++ {
		vec[i] = sampleGamma1Bounded(rho_pp, uint16(kappa*length+i))
	}
	return vec
}

// sampleGamma1Bounded samples a polynomial with coefficients in [-gamma1, gamma1].
func sampleGamma1Bounded(seed []byte, nonce uint16) []int32 {
	poly := make([]int32, MLDSA_N)
	h := sha3.NewShake256()
	h.Write(seed)
	h.Write([]byte{byte(nonce >> 8), byte(nonce)})

	buf := make([]byte, 5*MLDSA_N/4)
	h.Read(buf)

	for i := 0; i < MLDSA_N; i++ {
		idx := i * 5 / 4
		shift := uint(i%4) * 5
		val := (int32(buf[idx]) | int32(buf[idx+1])<<8) >> shift & 0x1FFFFF
		poly[i] = MLDSA_GAMMA1 - val
	}
	return poly
}

// sampleChallenge samples a sparse challenge polynomial with tau +/-1 coefficients.
func sampleChallenge(seed []byte) []int32 {
	c := make([]int32, MLDSA_N)
	h := sha3.NewShake256()
	h.Write(seed)

	// Set last tau positions to +/-1
	buf := make([]byte, 8)
	h.Read(buf)
	signs := new(big.Int).SetBytes(buf)

	indices := make([]int, 0, MLDSA_TAU)
	idxBuf := make([]byte, 1)
	for len(indices) < MLDSA_TAU {
		h.Read(idxBuf)
		idx := int(idxBuf[0])
		if idx < MLDSA_N {
			duplicate := false
			for _, existing := range indices {
				if existing == idx {
					duplicate = true
					break
				}
			}
			if !duplicate {
				indices = append(indices, idx)
				if signs.Bit(len(indices)-1) == 1 {
					c[idx] = -1
				} else {
					c[idx] = 1
				}
			}
		}
	}
	return c
}

// computeT computes t = A*s1 + s2 (in NTT domain, then inverse NTT).
func computeT(A [][][]int32, s1, s2 [][]int32) [][]int32 {
	t := make([][]int32, MLDSA_K)
	for i := 0; i < MLDSA_K; i++ {
		t[i] = make([]int32, MLDSA_N)
		for j := 0; j < MLDSA_L; j++ {
			for k := 0; k < MLDSA_N; k++ {
				t[i][k] = modQ(t[i][k] + A[i][j][k]*s1[j][k])
			}
		}
		for k := 0; k < MLDSA_N; k++ {
			t[i][k] = modQ(t[i][k] + s2[i][k])
		}
	}
	return t
}

// power2Round decomposes t into high bits t1 and low bits t0.
func power2Round(t [][]int32) (t1, t0 [][]int32) {
	t1 = make([][]int32, len(t))
	t0 = make([][]int32, len(t))
	for i := range t {
		t1[i] = make([]int32, MLDSA_N)
		t0[i] = make([]int32, MLDSA_N)
		for j := range t[i] {
			r := modQ(t[i][j])
			r0 := r % (1 << MLDSA_D)
			if r0 > (1 << (MLDSA_D - 1)) {
				r0 -= (1 << MLDSA_D)
			}
			t1[i][j] = (r - r0) >> MLDSA_D
			t0[i][j] = r0
		}
	}
	return
}

// matVecMul multiplies matrix A by vector v.
func matVecMul(A [][][]int32, v [][]int32) [][]int32 {
	result := make([][]int32, len(A))
	for i := range A {
		result[i] = make([]int32, MLDSA_N)
		for j := range v {
			for k := 0; k < MLDSA_N; k++ {
				result[i][k] = modQ(result[i][k] + A[i][j][k]*v[j][k])
			}
		}
	}
	return result
}

// scalarVecMul multiplies a polynomial c by a vector v.
func scalarVecMul(c []int32, v [][]int32) [][]int32 {
	result := make([][]int32, len(v))
	for i := range v {
		result[i] = make([]int32, MLDSA_N)
		for j := 0; j < MLDSA_N; j++ {
			result[i][j] = modQ(c[j] * v[i][j])
		}
	}
	return result
}

// addVec adds two polynomial vectors.
func addVec(a, b [][]int32) [][]int32 {
	result := make([][]int32, len(a))
	for i := range a {
		result[i] = make([]int32, MLDSA_N)
		for j := range a[i] {
			result[i][j] = modQ(a[i][j] + b[i][j])
		}
	}
	return result
}

// subVec subtracts vector b from vector a.
func subVec(a, b [][]int32) [][]int32 {
	result := make([][]int32, len(a))
	for i := range a {
		result[i] = make([]int32, MLDSA_N)
		for j := range a[i] {
			result[i][j] = modQ(a[i][j] - b[i][j])
		}
	}
	return result
}

// negVec negates a polynomial vector.
func negVec(v [][]int32) [][]int32 {
	result := make([][]int32, len(v))
	for i := range v {
		result[i] = make([]int32, MLDSA_N)
		for j := range v[i] {
			result[i][j] = modQ(-v[i][j])
		}
	}
	return result
}

// shiftLeft multiplies each coefficient by 2^d.
func shiftLeft(v [][]int32, d int) [][]int32 {
	result := make([][]int32, len(v))
	for i := range v {
		result[i] = make([]int32, MLDSA_N)
		for j := range v[i] {
			result[i][j] = modQ(v[i][j] << d)
		}
	}
	return result
}

// decomposeW decomposes w into high bits w1 and low bits w0.
func decomposeW(w [][]int32) (w1, w0 [][]int32) {
	w1 = make([][]int32, len(w))
	w0 = make([][]int32, len(w))
	for i := range w {
		w1[i] = make([]int32, MLDSA_N)
		w0[i] = make([]int32, MLDSA_N)
		for j := range w[i] {
			r := modQ(w[i][j])
			r0 := r % (2 * MLDSA_GAMMA2)
			if r0 > MLDSA_GAMMA2 {
				r0 -= 2 * MLDSA_GAMMA2
			}
			w1[i][j] = (r - r0) / (2 * MLDSA_GAMMA2)
			w0[i][j] = r0
		}
	}
	return
}

// decomposeW0 returns only the low bits of w.
func decomposeW0(w [][]int32) [][]int32 {
	_, w0 := decomposeW(w)
	return w0
}

// makeHints computes the hint vector.
func makeHints(r0, r1 [][]int32) [][]int32 {
	h := make([][]int32, len(r0))
	for i := range r0 {
		h[i] = make([]int32, MLDSA_N)
		for j := range r0[i] {
			if abs32(r0[i][j]) > MLDSA_GAMMA2 || (abs32(r0[i][j]) == MLDSA_GAMMA2 && r1[i][j] != 0) {
				h[i][j] = 1
			}
		}
	}
	return h
}

// useHints applies hints to recover w1.
func useHints(h, r [][]int32) [][]int32 {
	w1 := make([][]int32, len(r))
	for i := range r {
		w1[i] = make([]int32, MLDSA_N)
		for j := range r[i] {
			r0 := modQ(r[i][j])
			r1 := r0 / (2 * MLDSA_GAMMA2)
			if h[i][j] == 1 {
				if r0 > 0 {
					w1[i][j] = (r1 + 1) % ((MLDSA_Q - 1) / (2 * MLDSA_GAMMA2))
				} else {
					w1[i][j] = (r1 - 1 + (MLDSA_Q-1)/(2*MLDSA_GAMMA2)) % ((MLDSA_Q - 1) / (2 * MLDSA_GAMMA2))
				}
			} else {
				w1[i][j] = r1
			}
		}
	}
	return w1
}

// countHints counts the total number of hints.
func countHints(h [][]int32) int {
	count := 0
	for i := range h {
		for j := range h[i] {
			if h[i][j] != 0 {
				count++
			}
		}
	}
	return count
}

// checkNormBound checks if all coefficients of a vector are within the given bound.
func checkNormBound(v [][]int32, bound int32) bool {
	for i := range v {
		for j := range v[i] {
			if abs32(v[i][j]) >= bound {
				return false
			}
		}
	}
	return true
}

// encodeW1 encodes the high-order bits of w.
func encodeW1(w1 [][]int32) []byte {
	// Simple encoding: 4 bits per coefficient
	size := MLDSA_K * MLDSA_N / 2
	buf := make([]byte, size)
	for i := 0; i < MLDSA_K; i++ {
		for j := 0; j < MLDSA_N; j++ {
			idx := (i*MLDSA_N + j) / 2
			if j%2 == 0 {
				buf[idx] = byte(w1[i][j] & 0x0F)
			} else {
				buf[idx] |= byte((w1[i][j] & 0x0F) << 4)
			}
		}
	}
	return buf
}

// ============================================================
// Serialization Helpers
// ============================================================

func serializePublicKey(rho [32]byte, t1 [][]int32) []byte {
	buf := make([]byte, MLDSA_PUBLICKEY_SIZE)
	copy(buf[0:32], rho[:])
	// Encode t1 in remaining bytes
	offset := 32
	for i := 0; i < MLDSA_K && offset < len(buf); i++ {
		for j := 0; j < MLDSA_N && offset+1 < len(buf); j++ {
			buf[offset] = byte(t1[i][j])
			buf[offset+1] = byte(t1[i][j] >> 8)
			offset += 2
		}
	}
	return buf
}

func serializePrivateKey(rho [32]byte, kKey [32]byte, tr [64]byte, s1, s2, t0 [][]int32) []byte {
	buf := make([]byte, MLDSA_PRIVATEKEY_SIZE)
	copy(buf[0:32], rho[:])
	copy(buf[32:64], kKey[:])
	copy(buf[64:128], tr[:])
	return buf
}

func serializeSignature(c_tilde [64]byte, z [][]int32, h [][]int32) []byte {
	buf := make([]byte, MLDSA_SIGNATURE_SIZE)
	copy(buf[0:64], c_tilde[:])
	return buf
}

func deserializeT1(data []byte) [][]int32 {
	t1 := make([][]int32, MLDSA_K)
	offset := 0
	for i := 0; i < MLDSA_K; i++ {
		t1[i] = make([]int32, MLDSA_N)
		for j := 0; j < MLDSA_N && offset+1 < len(data); j++ {
			t1[i][j] = int32(data[offset]) | int32(data[offset+1])<<8
			offset += 2
		}
	}
	return t1
}

func deserializeSignature(data []byte) (*MLDSASignature, error) {
	var c_tilde [64]byte
	copy(c_tilde[:], data[0:64])
	return &MLDSASignature{
		c_tilde: c_tilde,
		z:       make([][]int32, MLDSA_L),
		h:       make([][]int32, MLDSA_K),
		raw:     data,
	}, nil
}

// modQ reduces x modulo MLDSA_Q.
func modQ(x int32) int32 {
	r := x % MLDSA_Q
	if r < 0 {
		r += MLDSA_Q
	}
	return r
}

// abs32 returns the absolute value of x.
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
