module github.com/NikaHsaini/Quantum-Blockchain-Pro/qbp-chain

go 1.22.3

require (
// Ethereum core - base for QBP fork
github.com/ethereum/go-ethereum v1.14.0

// CLI framework
github.com/urfave/cli/v2 v2.27.1
)

// Post-quantum cryptography is implemented natively in the crypto/pqcrypto package
// using pure Go, based on the NIST FIPS 204 (ML-DSA) and FIPS 203 (ML-KEM) standards.
// No external PQC library is required for the core implementation.
