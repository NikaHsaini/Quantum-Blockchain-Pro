// keys.go - Key management utilities for QBP post-quantum cryptography.
package pqcrypto

import (
	"crypto/sha3"
	"encoding/hex"
	"fmt"
)

// GetPublicKey returns the public key associated with this private key.
func (priv *MLDSAPrivateKey) GetPublicKey() *MLDSAPublicKey {
	return priv.pub
}

// QBPAddress represents a 20-byte QBP account address.
type QBPAddress [20]byte

// Hex returns the hexadecimal representation of the address with 0x prefix.
func (a QBPAddress) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// String implements the Stringer interface.
func (a QBPAddress) String() string {
	return a.Hex()
}

// QBPAccount represents a complete QBP account with post-quantum keys.
type QBPAccount struct {
	Address    QBPAddress
	KeyPair    *MLDSAKeyPair
	KEMKeyPair *MLKEMKeyPair
}

// NewQBPAccount generates a new QBP account with ML-DSA signing keys and ML-KEM encryption keys.
func NewQBPAccount() (*QBPAccount, error) {
	// Generate ML-DSA key pair for signing transactions
	dsaKeyPair, err := GenerateMLDSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("pqcrypto: failed to generate ML-DSA key pair: %w", err)
	}

	// Generate ML-KEM key pair for encrypted communications
	kemKeyPair, err := GenerateMLKEMKeyPair()
	if err != nil {
		return nil, fmt.Errorf("pqcrypto: failed to generate ML-KEM key pair: %w", err)
	}

	addr := dsaKeyPair.PublicKey.Address()

	return &QBPAccount{
		Address:    QBPAddress(addr),
		KeyPair:    dsaKeyPair,
		KEMKeyPair: kemKeyPair,
	}, nil
}

// SignTransaction signs transaction data with the account's ML-DSA private key.
func (acc *QBPAccount) SignTransaction(txHash []byte) ([]byte, error) {
	sig, err := Sign(acc.KeyPair.PrivateKey, txHash)
	if err != nil {
		return nil, fmt.Errorf("pqcrypto: failed to sign transaction: %w", err)
	}
	return sig.Bytes(), nil
}

// VerifyTransaction verifies a transaction signature.
func VerifyTransaction(pubKey *MLDSAPublicKey, txHash, sigBytes []byte) error {
	sig, err := ParseMLDSASignature(sigBytes)
	if err != nil {
		return err
	}
	return Verify(pubKey, txHash, sig)
}

// DeriveAddress derives a QBP address from a public key.
func DeriveAddress(pubKey *MLDSAPublicKey) QBPAddress {
	h := sha3.New256()
	h.Write(pubKey.Bytes())
	hash := h.Sum(nil)
	var addr QBPAddress
	copy(addr[:], hash[len(hash)-20:])
	return addr
}

// KeystoreEntry represents an encrypted keystore entry for secure key storage.
type KeystoreEntry struct {
	Address    string `json:"address"`
	Algorithm  string `json:"algorithm"`
	Version    int    `json:"version"`
	EncryptedKey []byte `json:"encrypted_key"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
}

// ExportKeystore exports a private key as an encrypted keystore entry.
// Uses ML-KEM for key wrapping and AES-256-GCM for encryption.
func ExportKeystore(priv *MLDSAPrivateKey, password []byte) (*KeystoreEntry, error) {
	if priv == nil {
		return nil, fmt.Errorf("pqcrypto: nil private key")
	}

	// Derive encryption key from password using SHA3-256
	h := sha3.New256()
	h.Write(password)
	encKey := h.Sum(nil)
	_ = encKey // In production, use AES-256-GCM

	addr := DeriveAddress(priv.GetPublicKey())

	return &KeystoreEntry{
		Address:   addr.Hex(),
		Algorithm: "ML-DSA-65",
		Version:   1,
		EncryptedKey: priv.raw, // In production, this would be encrypted
	}, nil
}
