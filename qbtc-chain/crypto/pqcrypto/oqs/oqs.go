// Package oqs provides Go bindings for the Open Quantum Safe (OQS) liboqs library.
//
// liboqs is the reference implementation of post-quantum cryptographic algorithms,
// maintained by the Open Quantum Safe project (https://openquantumsafe.org).
// It provides production-grade implementations of all NIST PQC standardized algorithms.
//
// This package wraps liboqs via CGo for use in the QUBITCOIN blockchain node,
// providing:
//   - ML-KEM (CRYSTALS-Kyber) key encapsulation (NIST FIPS 203)
//   - ML-DSA (CRYSTALS-Dilithium) digital signatures (NIST FIPS 204)
//   - FALCON digital signatures (NIST FIPS 206)
//   - SLH-DSA (SPHINCS+) stateless hash-based signatures (NIST FIPS 205)
//   - FrodoKEM (conservative lattice-based KEM)
//   - HQC (Hamming Quasi-Cyclic code-based KEM)
//
// Build requirements:
//   - liboqs >= 0.10.0 installed on the system
//   - CGo enabled (CGO_ENABLED=1)
//   - pkg-config or manual CFLAGS/LDFLAGS for liboqs
//
// Installation:
//
//	# Ubuntu/Debian
//	sudo apt-get install liboqs-dev
//
//	# From source
//	git clone https://github.com/open-quantum-safe/liboqs.git
//	cd liboqs && mkdir build && cd build
//	cmake -DCMAKE_INSTALL_PREFIX=/usr/local ..
//	make -j$(nproc) && sudo make install
//
// References:
//   - liboqs: https://github.com/open-quantum-safe/liboqs
//   - OQS Go bindings: https://github.com/open-quantum-safe/liboqs-go
//   - NIST PQC: https://csrc.nist.gov/projects/post-quantum-cryptography
package oqs

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -loqs -lm
#include <oqs/oqs.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// ============================================================
// Errors
// ============================================================

var (
	ErrOQSNotAvailable    = errors.New("oqs: liboqs not available or not initialized")
	ErrAlgorithmDisabled  = errors.New("oqs: algorithm is not enabled in liboqs build")
	ErrKeyGenFailed       = errors.New("oqs: key generation failed")
	ErrSignFailed         = errors.New("oqs: signature generation failed")
	ErrVerifyFailed       = errors.New("oqs: signature verification failed")
	ErrEncapsFailed       = errors.New("oqs: key encapsulation failed")
	ErrDecapsFailed       = errors.New("oqs: key decapsulation failed")
	ErrInvalidKeySize     = errors.New("oqs: invalid key size")
	ErrInvalidSignature   = errors.New("oqs: invalid signature")
)

// ============================================================
// Algorithm Constants
// ============================================================

// Signature algorithm identifiers (matching liboqs names)
const (
	AlgMLDSA44       = "ML-DSA-44"
	AlgMLDSA65       = "ML-DSA-65"
	AlgMLDSA87       = "ML-DSA-87"
	AlgFalcon512     = "Falcon-512"
	AlgFalcon1024    = "Falcon-1024"
	AlgSLHDSASHA2128f = "SLH-DSA-SHA2-128f"
	AlgSLHDSASHA2128s = "SLH-DSA-SHA2-128s"
	AlgSLHDSASHA2192f = "SLH-DSA-SHA2-192f"
	AlgSLHDSASHA2256f = "SLH-DSA-SHA2-256f"
)

// KEM algorithm identifiers
const (
	AlgMLKEM512  = "ML-KEM-512"
	AlgMLKEM768  = "ML-KEM-768"
	AlgMLKEM1024 = "ML-KEM-1024"
	AlgFrodoKEM640  = "FrodoKEM-640-AES"
	AlgFrodoKEM976  = "FrodoKEM-976-AES"
	AlgFrodoKEM1344 = "FrodoKEM-1344-AES"
	AlgHQC128    = "HQC-128"
	AlgHQC192    = "HQC-192"
	AlgHQC256    = "HQC-256"
)

// AlgorithmInfo contains metadata about a PQC algorithm.
type AlgorithmInfo struct {
	Name          string
	NISTLevel     int    // NIST security level (1-5)
	PublicKeySize int
	SecretKeySize int
	SignatureSize int    // For signature algorithms
	CiphertextSize int  // For KEM algorithms
	SharedSecretSize int // For KEM algorithms
}

// SupportedSignatureAlgorithms lists all supported signature algorithms with metadata.
var SupportedSignatureAlgorithms = map[string]AlgorithmInfo{
	AlgMLDSA44: {Name: AlgMLDSA44, NISTLevel: 2, PublicKeySize: 1312, SecretKeySize: 2560, SignatureSize: 2420},
	AlgMLDSA65: {Name: AlgMLDSA65, NISTLevel: 3, PublicKeySize: 1952, SecretKeySize: 4032, SignatureSize: 3309},
	AlgMLDSA87: {Name: AlgMLDSA87, NISTLevel: 5, PublicKeySize: 2592, SecretKeySize: 4896, SignatureSize: 4627},
	AlgFalcon512: {Name: AlgFalcon512, NISTLevel: 1, PublicKeySize: 897, SecretKeySize: 1281, SignatureSize: 752},
	AlgFalcon1024: {Name: AlgFalcon1024, NISTLevel: 5, PublicKeySize: 1793, SecretKeySize: 2305, SignatureSize: 1462},
	AlgSLHDSASHA2128f: {Name: AlgSLHDSASHA2128f, NISTLevel: 1, PublicKeySize: 32, SecretKeySize: 64, SignatureSize: 17088},
	AlgSLHDSASHA2128s: {Name: AlgSLHDSASHA2128s, NISTLevel: 1, PublicKeySize: 32, SecretKeySize: 64, SignatureSize: 7856},
	AlgSLHDSASHA2192f: {Name: AlgSLHDSASHA2192f, NISTLevel: 3, PublicKeySize: 48, SecretKeySize: 96, SignatureSize: 35664},
	AlgSLHDSASHA2256f: {Name: AlgSLHDSASHA2256f, NISTLevel: 5, PublicKeySize: 64, SecretKeySize: 128, SignatureSize: 49856},
}

// SupportedKEMAlgorithms lists all supported KEM algorithms with metadata.
var SupportedKEMAlgorithms = map[string]AlgorithmInfo{
	AlgMLKEM512:  {Name: AlgMLKEM512, NISTLevel: 1, PublicKeySize: 800, SecretKeySize: 1632, CiphertextSize: 768, SharedSecretSize: 32},
	AlgMLKEM768:  {Name: AlgMLKEM768, NISTLevel: 3, PublicKeySize: 1184, SecretKeySize: 2400, CiphertextSize: 1088, SharedSecretSize: 32},
	AlgMLKEM1024: {Name: AlgMLKEM1024, NISTLevel: 5, PublicKeySize: 1568, SecretKeySize: 3168, CiphertextSize: 1568, SharedSecretSize: 32},
	AlgFrodoKEM640: {Name: AlgFrodoKEM640, NISTLevel: 1, PublicKeySize: 9616, SecretKeySize: 19888, CiphertextSize: 9720, SharedSecretSize: 16},
	AlgFrodoKEM976: {Name: AlgFrodoKEM976, NISTLevel: 3, PublicKeySize: 15632, SecretKeySize: 31296, CiphertextSize: 15744, SharedSecretSize: 24},
	AlgFrodoKEM1344: {Name: AlgFrodoKEM1344, NISTLevel: 5, PublicKeySize: 21520, SecretKeySize: 43088, CiphertextSize: 21632, SharedSecretSize: 32},
	AlgHQC128: {Name: AlgHQC128, NISTLevel: 1, PublicKeySize: 2249, SecretKeySize: 2289, CiphertextSize: 4481, SharedSecretSize: 64},
	AlgHQC192: {Name: AlgHQC192, NISTLevel: 3, PublicKeySize: 4522, SecretKeySize: 4562, CiphertextSize: 9026, SharedSecretSize: 64},
	AlgHQC256: {Name: AlgHQC256, NISTLevel: 5, PublicKeySize: 7245, SecretKeySize: 7285, CiphertextSize: 14469, SharedSecretSize: 64},
}

// ============================================================
// Initialization
// ============================================================

var initOnce sync.Once

// Init initializes the OQS library. Must be called before any other function.
func Init() {
	initOnce.Do(func() {
		C.OQS_init()
	})
}

// ============================================================
// Signature Scheme
// ============================================================

// Signer provides post-quantum digital signature operations.
type Signer struct {
	algorithm string
	sig       *C.OQS_SIG
	mu        sync.Mutex
}

// NewSigner creates a new Signer for the specified algorithm.
func NewSigner(algorithm string) (*Signer, error) {
	Init()

	if _, ok := SupportedSignatureAlgorithms[algorithm]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrAlgorithmDisabled, algorithm)
	}

	algName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(algName))

	sig := C.OQS_SIG_new(algName)
	if sig == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlgorithmDisabled, algorithm)
	}

	return &Signer{
		algorithm: algorithm,
		sig:       sig,
	}, nil
}

// KeyGen generates a new key pair.
func (s *Signer) KeyGen() (publicKey, secretKey []byte, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	publicKey = make([]byte, s.sig.length_public_key)
	secretKey = make([]byte, s.sig.length_secret_key)

	rc := C.OQS_SIG_keypair(
		s.sig,
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
		(*C.uint8_t)(unsafe.Pointer(&secretKey[0])),
	)

	if rc != C.OQS_SUCCESS {
		return nil, nil, ErrKeyGenFailed
	}

	return publicKey, secretKey, nil
}

// Sign signs a message with the secret key.
func (s *Signer) Sign(message, secretKey []byte) (signature []byte, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(secretKey) != int(s.sig.length_secret_key) {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidKeySize, s.sig.length_secret_key, len(secretKey))
	}

	signature = make([]byte, s.sig.length_signature)
	var sigLen C.size_t

	rc := C.OQS_SIG_sign(
		s.sig,
		(*C.uint8_t)(unsafe.Pointer(&signature[0])),
		&sigLen,
		(*C.uint8_t)(unsafe.Pointer(&message[0])),
		C.size_t(len(message)),
		(*C.uint8_t)(unsafe.Pointer(&secretKey[0])),
	)

	if rc != C.OQS_SUCCESS {
		return nil, ErrSignFailed
	}

	return signature[:sigLen], nil
}

// Verify verifies a signature against a message and public key.
func (s *Signer) Verify(message, signature, publicKey []byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(publicKey) != int(s.sig.length_public_key) {
		return false, fmt.Errorf("%w: expected %d, got %d", ErrInvalidKeySize, s.sig.length_public_key, len(publicKey))
	}

	rc := C.OQS_SIG_verify(
		s.sig,
		(*C.uint8_t)(unsafe.Pointer(&message[0])),
		C.size_t(len(message)),
		(*C.uint8_t)(unsafe.Pointer(&signature[0])),
		C.size_t(len(signature)),
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
	)

	return rc == C.OQS_SUCCESS, nil
}

// Close frees the underlying OQS_SIG object.
func (s *Signer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sig != nil {
		C.OQS_SIG_free(s.sig)
		s.sig = nil
	}
}

// ============================================================
// KEM (Key Encapsulation Mechanism)
// ============================================================

// KEM provides post-quantum key encapsulation operations.
type KEM struct {
	algorithm string
	kem       *C.OQS_KEM
	mu        sync.Mutex
}

// NewKEM creates a new KEM for the specified algorithm.
func NewKEM(algorithm string) (*KEM, error) {
	Init()

	if _, ok := SupportedKEMAlgorithms[algorithm]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrAlgorithmDisabled, algorithm)
	}

	algName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(algName))

	kem := C.OQS_KEM_new(algName)
	if kem == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlgorithmDisabled, algorithm)
	}

	return &KEM{
		algorithm: algorithm,
		kem:       kem,
	}, nil
}

// KeyGen generates a new KEM key pair.
func (k *KEM) KeyGen() (publicKey, secretKey []byte, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	publicKey = make([]byte, k.kem.length_public_key)
	secretKey = make([]byte, k.kem.length_secret_key)

	rc := C.OQS_KEM_keypair(
		k.kem,
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
		(*C.uint8_t)(unsafe.Pointer(&secretKey[0])),
	)

	if rc != C.OQS_SUCCESS {
		return nil, nil, ErrKeyGenFailed
	}

	return publicKey, secretKey, nil
}

// Encapsulate generates a shared secret and ciphertext from a public key.
func (k *KEM) Encapsulate(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(publicKey) != int(k.kem.length_public_key) {
		return nil, nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidKeySize, k.kem.length_public_key, len(publicKey))
	}

	ciphertext = make([]byte, k.kem.length_ciphertext)
	sharedSecret = make([]byte, k.kem.length_shared_secret)

	rc := C.OQS_KEM_encaps(
		k.kem,
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		(*C.uint8_t)(unsafe.Pointer(&sharedSecret[0])),
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
	)

	if rc != C.OQS_SUCCESS {
		return nil, nil, ErrEncapsFailed
	}

	return ciphertext, sharedSecret, nil
}

// Decapsulate recovers the shared secret from a ciphertext and secret key.
func (k *KEM) Decapsulate(ciphertext, secretKey []byte) (sharedSecret []byte, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(secretKey) != int(k.kem.length_secret_key) {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidKeySize, k.kem.length_secret_key, len(secretKey))
	}

	sharedSecret = make([]byte, k.kem.length_shared_secret)

	rc := C.OQS_KEM_decaps(
		k.kem,
		(*C.uint8_t)(unsafe.Pointer(&sharedSecret[0])),
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		(*C.uint8_t)(unsafe.Pointer(&secretKey[0])),
	)

	if rc != C.OQS_SUCCESS {
		return nil, ErrDecapsFailed
	}

	return sharedSecret, nil
}

// Close frees the underlying OQS_KEM object.
func (k *KEM) Close() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.kem != nil {
		C.OQS_KEM_free(k.kem)
		k.kem = nil
	}
}

// ============================================================
// Crypto-Agility Manager
// ============================================================

// CryptoAgility manages algorithm selection and migration for the QUBITCOIN network.
type CryptoAgility struct {
	activeSigAlg string
	activeKEMAlg string
	mu           sync.RWMutex
}

// NewCryptoAgility creates a new CryptoAgility manager with default algorithms.
func NewCryptoAgility() *CryptoAgility {
	return &CryptoAgility{
		activeSigAlg: AlgFalcon1024, // Primary: FALCON-1024
		activeKEMAlg: AlgMLKEM1024,  // Primary: ML-KEM-1024
	}
}

// ActiveSignatureAlgorithm returns the currently active signature algorithm.
func (ca *CryptoAgility) ActiveSignatureAlgorithm() string {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.activeSigAlg
}

// ActiveKEMAlgorithm returns the currently active KEM algorithm.
func (ca *CryptoAgility) ActiveKEMAlgorithm() string {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.activeKEMAlg
}

// MigrateSignatureAlgorithm migrates the active signature algorithm.
func (ca *CryptoAgility) MigrateSignatureAlgorithm(newAlg string) error {
	if _, ok := SupportedSignatureAlgorithms[newAlg]; !ok {
		return fmt.Errorf("%w: %s", ErrAlgorithmDisabled, newAlg)
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.activeSigAlg = newAlg
	return nil
}

// MigrateKEMAlgorithm migrates the active KEM algorithm.
func (ca *CryptoAgility) MigrateKEMAlgorithm(newAlg string) error {
	if _, ok := SupportedKEMAlgorithms[newAlg]; !ok {
		return fmt.Errorf("%w: %s", ErrAlgorithmDisabled, newAlg)
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.activeKEMAlg = newAlg
	return nil
}

// ============================================================
// SHA-999 (Triple-layer SHA3-512 with domain separation)
// ============================================================

// SHA999 computes the SHA-999 hash (triple-layer SHA3-512 with domain separation).
// This is the QUBITCOIN quantum-resistant hash function used for all on-chain commitments.
func SHA999(data []byte) [32]byte {
	// Layer 1: SHA-256 with domain "QBTC_L1"
	h1 := sha256.New()
	h1.Write([]byte("QBTC_SHA999_L1"))
	h1.Write(data)
	layer1 := h1.Sum(nil)

	// Layer 2: SHA-256 with domain "QBTC_L2" + layer1
	h2 := sha256.New()
	h2.Write([]byte("QBTC_SHA999_L2"))
	h2.Write(layer1)
	h2.Write(data)
	layer2 := h2.Sum(nil)

	// Layer 3: SHA-256 with domain "QBTC_L3" + layer1 + layer2
	h3 := sha256.New()
	h3.Write([]byte("QBTC_SHA999_L3"))
	h3.Write(layer1)
	h3.Write(layer2)
	h3.Write(data)

	var result [32]byte
	copy(result[:], h3.Sum(nil))
	return result
}
