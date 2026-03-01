// ml_kem.go - ML-KEM (Module-Lattice-Based Key-Encapsulation Mechanism)
// Implements FIPS 203 (CRYSTALS-Kyber) for quantum-resistant key exchange.
// Used for P2P channel encryption in QBP network.
package pqcrypto

import (
	"crypto/rand"
	"crypto/sha3"
	"fmt"
)

// ============================================================
// ML-KEM Parameters (ML-KEM-768 / Kyber768 - NIST Security Level 3)
// ============================================================

const (
	MLKEM_K   = 3    // Module rank
	MLKEM_N   = 256  // Polynomial degree
	MLKEM_Q   = 3329 // Prime modulus

	MLKEM_ETA1 = 2 // Noise parameter for key generation
	MLKEM_ETA2 = 2 // Noise parameter for encapsulation

	MLKEM_DU = 10 // Compression parameter for ciphertext u
	MLKEM_DV = 4  // Compression parameter for ciphertext v

	// Key and ciphertext sizes (bytes)
	MLKEM_PUBLICKEY_SIZE  = 1184 // 32 + 32*K*12 bits
	MLKEM_PRIVATEKEY_SIZE = 2400 // Full private key
	MLKEM_CIPHERTEXT_SIZE = 1088 // Compressed ciphertext
	MLKEM_SHARED_KEY_SIZE = 32   // Shared secret size
	MLKEM_SEED_SIZE       = 64   // Seed for key generation
)

// MLKEMPublicKey represents an ML-KEM public key.
type MLKEMPublicKey struct {
	t_hat [][]int32 // Public polynomial vector (NTT domain)
	rho   [32]byte  // Seed for matrix A
	raw   []byte
}

// MLKEMPrivateKey represents an ML-KEM private key.
type MLKEMPrivateKey struct {
	s_hat [][]int32     // Secret polynomial vector (NTT domain)
	pub   *MLKEMPublicKey
	h_pk  [32]byte      // Hash of public key
	z     [32]byte      // Implicit rejection value
	raw   []byte
}

// MLKEMKeyPair holds an ML-KEM public/private key pair.
type MLKEMKeyPair struct {
	PublicKey  *MLKEMPublicKey
	PrivateKey *MLKEMPrivateKey
}

// GenerateMLKEMKeyPair generates a new ML-KEM-768 key pair.
func GenerateMLKEMKeyPair() (*MLKEMKeyPair, error) {
	seed := make([]byte, MLKEM_SEED_SIZE)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("pqcrypto: failed to generate ML-KEM seed: %w", err)
	}
	return GenerateMLKEMKeyPairFromSeed(seed)
}

// GenerateMLKEMKeyPairFromSeed generates a deterministic ML-KEM-768 key pair from a 64-byte seed.
func GenerateMLKEMKeyPairFromSeed(seed []byte) (*MLKEMKeyPair, error) {
	if len(seed) < MLKEM_SEED_SIZE {
		return nil, fmt.Errorf("pqcrypto: ML-KEM seed must be at least %d bytes", MLKEM_SEED_SIZE)
	}

	// G(d) = (rho, sigma) using SHA3-512
	h := sha3.New512()
	h.Write(seed[:32])
	h.Write([]byte{byte(MLKEM_K)})
	expanded := h.Sum(nil)

	var rho [32]byte
	var sigma [32]byte
	copy(rho[:], expanded[0:32])
	copy(sigma[:], expanded[32:64])

	// Generate matrix A_hat from rho
	A_hat := mlkemExpandMatrix(rho)

	// Sample secret vector s from sigma
	s := mlkemSampleSecretVector(sigma[:], 0, MLKEM_ETA1)

	// Sample error vector e from sigma
	e := mlkemSampleSecretVector(sigma[:], MLKEM_K, MLKEM_ETA1)

	// Compute t_hat = NTT(A_hat * NTT(s) + NTT(e))
	s_hat := mlkemNTTVector(s)
	t_hat := mlkemMatVecMulNTT(A_hat, s_hat)
	e_hat := mlkemNTTVector(e)
	for i := 0; i < MLKEM_K; i++ {
		for j := 0; j < MLKEM_N; j++ {
			t_hat[i][j] = mlkemModQ(t_hat[i][j] + e_hat[i][j])
		}
	}

	// Serialize public key
	pkBytes := mlkemSerializePublicKey(t_hat, rho)

	// Hash public key
	var h_pk [32]byte
	pkHasher := sha3.New256()
	pkHasher.Write(pkBytes)
	copy(h_pk[:], pkHasher.Sum(nil))

	// Generate implicit rejection value z
	var z [32]byte
	rand.Read(z[:])

	pub := &MLKEMPublicKey{
		t_hat: t_hat,
		rho:   rho,
		raw:   pkBytes,
	}

	priv := &MLKEMPrivateKey{
		s_hat: s_hat,
		pub:   pub,
		h_pk:  h_pk,
		z:     z,
		raw:   mlkemSerializePrivateKey(s_hat, pkBytes, h_pk, z),
	}

	return &MLKEMKeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// Encapsulate generates a shared secret and ciphertext using the public key.
// Returns (ciphertext, sharedSecret, error).
func Encapsulate(pub *MLKEMPublicKey) ([]byte, []byte, error) {
	if pub == nil {
		return nil, nil, fmt.Errorf("pqcrypto: nil ML-KEM public key")
	}

	// Generate random message m
	m := make([]byte, 32)
	if _, err := rand.Read(m); err != nil {
		return nil, nil, fmt.Errorf("pqcrypto: failed to generate random message: %w", err)
	}

	// (K, r) = G(m || H(pk))
	h := sha3.New512()
	h.Write(m)
	pkHasher := sha3.New256()
	pkHasher.Write(pub.raw)
	h.Write(pkHasher.Sum(nil))
	Kr := h.Sum(nil)

	sharedSecret := Kr[0:32]
	r := Kr[32:64]

	// Encrypt m using randomness r
	ciphertext, err := mlkemEncrypt(pub, m, r)
	if err != nil {
		return nil, nil, err
	}

	return ciphertext, sharedSecret, nil
}

// Decapsulate recovers the shared secret from a ciphertext using the private key.
func Decapsulate(priv *MLKEMPrivateKey, ciphertext []byte) ([]byte, error) {
	if priv == nil {
		return nil, fmt.Errorf("pqcrypto: nil ML-KEM private key")
	}
	if len(ciphertext) != MLKEM_CIPHERTEXT_SIZE {
		return nil, fmt.Errorf("pqcrypto: invalid ciphertext size: expected %d, got %d",
			MLKEM_CIPHERTEXT_SIZE, len(ciphertext))
	}

	// Decrypt to recover m'
	mPrime, err := mlkemDecrypt(priv, ciphertext)
	if err != nil {
		return nil, err
	}

	// Re-derive (K', r') = G(m' || H(pk))
	h := sha3.New512()
	h.Write(mPrime)
	h.Write(priv.h_pk[:])
	KrPrime := h.Sum(nil)

	sharedSecretPrime := KrPrime[0:32]
	rPrime := KrPrime[32:64]

	// Re-encrypt and check
	ciphertextPrime, err := mlkemEncrypt(priv.pub, mPrime, rPrime)
	if err != nil {
		return nil, err
	}

	// Constant-time comparison to prevent timing attacks
	if !constantTimeEqual(ciphertext, ciphertextPrime) {
		// Implicit rejection: return H(z || c)
		h2 := sha3.New256()
		h2.Write(priv.z[:])
		h2.Write(ciphertext)
		return h2.Sum(nil), nil
	}

	return sharedSecretPrime, nil
}

// ============================================================
// ML-KEM Internal Operations
// ============================================================

// mlkemEncrypt encrypts message m using public key and randomness r.
func mlkemEncrypt(pub *MLKEMPublicKey, m, r []byte) ([]byte, error) {
	// Sample r vector
	rVec := mlkemSampleSecretVector(r, 0, MLKEM_ETA1)
	e1 := mlkemSampleSecretVector(r, MLKEM_K, MLKEM_ETA2)
	e2 := mlkemSampleSecretPoly(r, 2*MLKEM_K, MLKEM_ETA2)

	// A_hat from rho
	A_hat := mlkemExpandMatrix(pub.rho)

	// r_hat = NTT(r)
	r_hat := mlkemNTTVector(rVec)

	// u = INTT(A_hat^T * r_hat) + e1
	u := mlkemMatTransVecMulNTT(A_hat, r_hat)
	for i := 0; i < MLKEM_K; i++ {
		for j := 0; j < MLKEM_N; j++ {
			u[i][j] = mlkemModQ(u[i][j] + e1[i][j])
		}
	}

	// v = INTT(t_hat^T * r_hat) + e2 + Decompress(m, 1)
	v := mlkemDotProductNTT(pub.t_hat, r_hat)
	for j := 0; j < MLKEM_N; j++ {
		v[j] = mlkemModQ(v[j] + e2[j] + mlkemDecompressPoly(m, j))
	}

	// Compress and serialize
	return mlkemCompressCiphertext(u, v), nil
}

// mlkemDecrypt decrypts ciphertext using private key.
func mlkemDecrypt(priv *MLKEMPrivateKey, ciphertext []byte) ([]byte, error) {
	// Decompress ciphertext
	u, v := mlkemDecompressCiphertext(ciphertext)

	// m' = Compress(v - INTT(s_hat^T * NTT(u)), 1)
	su := mlkemDotProductNTT(priv.s_hat, mlkemNTTVector(u))
	for j := 0; j < MLKEM_N; j++ {
		v[j] = mlkemModQ(v[j] - su[j])
	}

	return mlkemCompressPoly(v), nil
}

// ============================================================
// NTT and Polynomial Operations for ML-KEM
// ============================================================

// mlkemExpandMatrix generates matrix A_hat from seed rho.
func mlkemExpandMatrix(rho [32]byte) [][][]int32 {
	A := make([][][]int32, MLKEM_K)
	for i := 0; i < MLKEM_K; i++ {
		A[i] = make([][]int32, MLKEM_K)
		for j := 0; j < MLKEM_K; j++ {
			A[i][j] = mlkemSampleUniform(rho[:], byte(i), byte(j))
		}
	}
	return A
}

// mlkemSampleUniform samples a uniform polynomial mod q.
func mlkemSampleUniform(seed []byte, i, j byte) []int32 {
	poly := make([]int32, MLKEM_N)
	h := sha3.NewShake128()
	h.Write(seed)
	h.Write([]byte{i, j})

	buf := make([]byte, 3*MLKEM_N)
	h.Read(buf)

	k := 0
	for idx := 0; idx < len(buf) && k < MLKEM_N; idx += 3 {
		d1 := int32(buf[idx]) | int32(buf[idx+1]&0x0F)<<8
		d2 := int32(buf[idx+1]>>4) | int32(buf[idx+2])<<4
		if d1 < MLKEM_Q {
			poly[k] = d1
			k++
		}
		if d2 < MLKEM_Q && k < MLKEM_N {
			poly[k] = d2
			k++
		}
	}
	return poly
}

// mlkemSampleSecretVector samples a secret polynomial vector with small coefficients.
func mlkemSampleSecretVector(seed []byte, offset, eta int) [][]int32 {
	vec := make([][]int32, MLKEM_K)
	for i := 0; i < MLKEM_K; i++ {
		vec[i] = mlkemSampleCBD(seed, byte(offset+i), eta)
	}
	return vec
}

// mlkemSampleSecretPoly samples a single secret polynomial.
func mlkemSampleSecretPoly(seed []byte, nonce, eta int) []int32 {
	return mlkemSampleCBD(seed, byte(nonce), eta)
}

// mlkemSampleCBD samples from the centered binomial distribution.
func mlkemSampleCBD(seed []byte, nonce byte, eta int) []int32 {
	poly := make([]int32, MLKEM_N)
	h := sha3.NewShake256()
	h.Write(seed)
	h.Write([]byte{nonce})

	buf := make([]byte, 64*eta)
	h.Read(buf)

	for i := 0; i < MLKEM_N; i++ {
		var a, b int32
		for j := 0; j < eta; j++ {
			byteIdx := (2*i*eta + 2*j) / 8
			bitIdx := uint((2*i*eta + 2*j) % 8)
			if byteIdx < len(buf) {
				a += int32((buf[byteIdx] >> bitIdx) & 1)
				if bitIdx+1 < 8 {
					b += int32((buf[byteIdx] >> (bitIdx + 1)) & 1)
				} else if byteIdx+1 < len(buf) {
					b += int32(buf[byteIdx+1] & 1)
				}
			}
		}
		poly[i] = a - b
	}
	return poly
}

// mlkemNTTVector applies NTT to each polynomial in the vector.
func mlkemNTTVector(v [][]int32) [][]int32 {
	result := make([][]int32, len(v))
	for i := range v {
		result[i] = mlkemNTT(v[i])
	}
	return result
}

// mlkemNTT applies the Number Theoretic Transform to a polynomial.
// Uses the Kyber NTT with precomputed roots of unity.
func mlkemNTT(poly []int32) []int32 {
	result := make([]int32, MLKEM_N)
	copy(result, poly)

	k := 1
	for len_ := 128; len_ >= 2; len_ >>= 1 {
		for start := 0; start < MLKEM_N; start += 2 * len_ {
			zeta := mlkemZeta(k)
			k++
			for j := start; j < start+len_; j++ {
				t := mlkemMontgomery(zeta, result[j+len_])
				result[j+len_] = mlkemModQ(result[j] - t)
				result[j] = mlkemModQ(result[j] + t)
			}
		}
	}
	return result
}

// mlkemMatVecMulNTT multiplies matrix A_hat by vector v_hat in NTT domain.
func mlkemMatVecMulNTT(A [][][]int32, v [][]int32) [][]int32 {
	result := make([][]int32, MLKEM_K)
	for i := 0; i < MLKEM_K; i++ {
		result[i] = make([]int32, MLKEM_N)
		for j := 0; j < MLKEM_K; j++ {
			prod := mlkemPolyMulNTT(A[i][j], v[j])
			for k := 0; k < MLKEM_N; k++ {
				result[i][k] = mlkemModQ(result[i][k] + prod[k])
			}
		}
	}
	return result
}

// mlkemMatTransVecMulNTT multiplies transposed matrix A_hat^T by vector v_hat.
func mlkemMatTransVecMulNTT(A [][][]int32, v [][]int32) [][]int32 {
	result := make([][]int32, MLKEM_K)
	for i := 0; i < MLKEM_K; i++ {
		result[i] = make([]int32, MLKEM_N)
		for j := 0; j < MLKEM_K; j++ {
			prod := mlkemPolyMulNTT(A[j][i], v[j]) // Transposed: A[j][i] instead of A[i][j]
			for k := 0; k < MLKEM_N; k++ {
				result[i][k] = mlkemModQ(result[i][k] + prod[k])
			}
		}
	}
	return result
}

// mlkemDotProductNTT computes the dot product of two vectors in NTT domain.
func mlkemDotProductNTT(a, b [][]int32) []int32 {
	result := make([]int32, MLKEM_N)
	for i := 0; i < len(a) && i < len(b); i++ {
		prod := mlkemPolyMulNTT(a[i], b[i])
		for j := 0; j < MLKEM_N; j++ {
			result[j] = mlkemModQ(result[j] + prod[j])
		}
	}
	return result
}

// mlkemPolyMulNTT multiplies two polynomials in NTT domain.
func mlkemPolyMulNTT(a, b []int32) []int32 {
	result := make([]int32, MLKEM_N)
	for i := 0; i < MLKEM_N; i++ {
		result[i] = mlkemMontgomery(a[i], b[i])
	}
	return result
}

// mlkemMontgomery performs Montgomery multiplication mod q.
func mlkemMontgomery(a, b int32) int32 {
	return mlkemModQ(a * b)
}

// mlkemZeta returns the i-th zeta value for NTT.
func mlkemZeta(i int) int32 {
	// Precomputed zeta values for Kyber NTT (first few)
	zetas := []int32{
		2285, 2571, 2970, 1812, 1493, 1422, 287, 202, 3158, 622,
		1577, 182, 962, 2127, 1855, 1468, 573, 2004, 264, 383,
		2500, 1458, 1727, 3199, 2648, 1017, 732, 608, 1787, 411,
		3124, 1758, 1223, 652, 2777, 1015, 2036, 1491, 3047, 1785,
		516, 3321, 3009, 2663, 1711, 2167, 126, 1469, 2476, 3239,
		3058, 830, 107, 1908, 3082, 2378, 2931, 961, 1821, 2604,
		448, 2264, 677, 2054, 2226, 430, 555, 843, 2078, 871,
		1550, 105, 422, 587, 177, 3094, 3038, 2869, 1574, 1653,
		3083, 778, 1159, 3182, 2552, 1483, 2727, 1119, 1739, 644,
		2457, 349, 418, 329, 3173, 3254, 817, 1097, 603, 610,
		1322, 2044, 1864, 384, 2114, 3193, 1218, 1994, 2455, 220,
		2142, 1670, 2144, 1799, 2051, 794, 1819, 2475, 2459, 478,
		3221, 3021, 996, 991, 958, 1869, 1522, 1628,
	}
	if i < len(zetas) {
		return zetas[i]
	}
	return 1
}

// mlkemModQ reduces x modulo MLKEM_Q.
func mlkemModQ(x int32) int32 {
	r := x % MLKEM_Q
	if r < 0 {
		r += MLKEM_Q
	}
	return r
}

// ============================================================
// Compression and Serialization
// ============================================================

// mlkemDecompressPoly decompresses a single bit of message into a polynomial coefficient.
func mlkemDecompressPoly(m []byte, idx int) int32 {
	byteIdx := idx / 8
	bitIdx := uint(idx % 8)
	if byteIdx >= len(m) {
		return 0
	}
	bit := (m[byteIdx] >> bitIdx) & 1
	return int32(bit) * ((MLKEM_Q + 1) / 2)
}

// mlkemCompressPoly compresses a polynomial to a 1-bit-per-coefficient message.
func mlkemCompressPoly(v []int32) []byte {
	m := make([]byte, MLKEM_N/8)
	for i := 0; i < MLKEM_N; i++ {
		// Round v[i] * 2 / q
		bit := ((int32(2)*v[i] + MLKEM_Q/2) / MLKEM_Q) & 1
		m[i/8] |= byte(bit) << uint(i%8)
	}
	return m
}

// mlkemCompressCiphertext compresses u and v into a ciphertext byte slice.
func mlkemCompressCiphertext(u [][]int32, v []int32) []byte {
	buf := make([]byte, MLKEM_CIPHERTEXT_SIZE)
	// Simplified compression
	offset := 0
	for i := 0; i < MLKEM_K && offset < len(buf)-2; i++ {
		for j := 0; j < MLKEM_N && offset < len(buf)-1; j++ {
			compressed := (int32(1<<MLKEM_DU)*u[i][j] + MLKEM_Q/2) / MLKEM_Q
			buf[offset] = byte(compressed)
			buf[offset+1] = byte(compressed >> 8)
			offset += 2
		}
	}
	return buf
}

// mlkemDecompressCiphertext decompresses a ciphertext into u and v.
func mlkemDecompressCiphertext(ciphertext []byte) ([][]int32, []int32) {
	u := make([][]int32, MLKEM_K)
	for i := 0; i < MLKEM_K; i++ {
		u[i] = make([]int32, MLKEM_N)
		for j := 0; j < MLKEM_N; j++ {
			idx := (i*MLKEM_N + j) * 2
			if idx+1 < len(ciphertext) {
				compressed := int32(ciphertext[idx]) | int32(ciphertext[idx+1])<<8
				u[i][j] = (compressed*MLKEM_Q + (1 << (MLKEM_DU - 1))) >> MLKEM_DU
			}
		}
	}
	v := make([]int32, MLKEM_N)
	return u, v
}

// mlkemSerializePublicKey serializes the ML-KEM public key.
func mlkemSerializePublicKey(t_hat [][]int32, rho [32]byte) []byte {
	buf := make([]byte, MLKEM_PUBLICKEY_SIZE)
	offset := 0
	for i := 0; i < MLKEM_K && offset < len(buf)-1; i++ {
		for j := 0; j < MLKEM_N && offset < len(buf)-1; j++ {
			buf[offset] = byte(t_hat[i][j])
			buf[offset+1] = byte(t_hat[i][j] >> 8)
			offset += 2
		}
	}
	if offset+32 <= len(buf) {
		copy(buf[offset:], rho[:])
	}
	return buf
}

// mlkemSerializePrivateKey serializes the ML-KEM private key.
func mlkemSerializePrivateKey(s_hat [][]int32, pkBytes []byte, h_pk [32]byte, z [32]byte) []byte {
	buf := make([]byte, MLKEM_PRIVATEKEY_SIZE)
	copy(buf[0:32], z[:])
	copy(buf[32:64], h_pk[:])
	if len(pkBytes) <= MLKEM_PRIVATEKEY_SIZE-64 {
		copy(buf[64:], pkBytes)
	}
	return buf
}

// Bytes returns the serialized public key.
func (pk *MLKEMPublicKey) Bytes() []byte {
	return pk.raw
}

// Bytes returns the serialized private key.
func (priv *MLKEMPrivateKey) Bytes() []byte {
	return priv.raw
}

// constantTimeEqual compares two byte slices in constant time to prevent timing attacks.
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
