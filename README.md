# QUBITCOIN (QBTC) — Quantum-Resistant Ethereum Fork

| Build Status | Security Audit | License | Chat |
| :--- | :--- | :--- | :--- |
| [![Go Build & Test](https://github.com/NikaHsaini/Quantum-Blockchain-Pro/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/NikaHsaini/Quantum-Blockchain-Pro/actions/workflows/ci.yml) | [![Slither](https://github.com/NikaHsaini/Quantum-Blockchain-Pro/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/NikaHsaini/Quantum-Blockchain-Pro/actions/workflows/ci.yml) | [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) | [![Discord](https://img.shields.io/discord/8?label=discord)](https://discord.gg/qubitcoin) |

**QUBITCOIN (QBTC)** is a quantum-resistant fork of Ethereum, designed to provide long-term security against both classical and quantum attacks. It integrates cutting-edge post-quantum cryptography (PQC) from the **ZKnox** team (funded by the Ethereum Foundation) and combines it with a novel Quantum Proof-of-Authority (QPoA) consensus and a useful mining mechanism based on quantum computation.

This project is not just a theoretical exercise; it is a production-grade implementation of a quantum-safe blockchain, built on the robust foundation of `go-ethereum` and designed for institutional adoption in the post-quantum era.

## Key Features

| Feature | Description | Technology Stack |
| :--- | :--- | :--- |
| **Quantum-Resistant Signatures** | Native support for FALCON, ML-DSA, and ML-KEM, the NIST-standardized PQC algorithms. | **ETHFALCON**, **ETHDilithium** (ZKnox), **EPERVIER** (ZKnox) |
| **Hybrid Accounts (EIP-7702)** | Accounts support both ECDSA (legacy) and FALCON (PQ) signatures for backward compatibility. | `QBTCAccount.sol` (inspired by ZKnox PQKINGS) |
| **ZK-SNARKs & Privacy** | Zero-knowledge proofs for private transactions and quantum-safe identity. | **gnark** (ConsenSys), **Poseidon2**, **Bandersnatch** (ZKnox) |
| **Quantum Proof-of-Authority (QPoA)** | A novel consensus mechanism where validators must prove quantum capability. | Custom Go implementation, `QPoARegistry.sol` |
| **Useful Quantum Mining** | Miners perform useful quantum computations (VQE, Grover, QFT) instead of arbitrary hashing. | `qminer.go`, `QuantumOracle.sol` |
| **qEVM (Quantum EVM)** | An extension of the EVM with 16 new opcodes for on-chain quantum computation. | `qevm.go` |
| **Professional Repository Structure** | Organized like `go-ethereum` with clear modules, CI/CD, tests, and documentation. | GitHub Actions, Foundry, Docker, Go tests |
| **Institutional Compliance** | Designed for compliance with MiCA, eIDAS 2.0, and DORA regulations. | `QBTCToken.sol` (AML/CFT blacklist, vesting) |

## Repository Structure

The repository is structured like a professional blockchain project, separating the core Go implementation from the Solidity contracts, tests, and documentation.

```
qbtc-chain/           # Core Go implementation (fork of go-ethereum)
├── cmd/qbtc/         # CLI entrypoint for the QBTC node
├── consensus/qpoa/   # Quantum Proof-of-Authority engine
├── core/vm/quantum/  # qEVM (Quantum EVM) with new opcodes
├── crypto/
│   ├── pqcrypto/     # Post-quantum crypto (FALCON, Dilithium, ML-KEM, NTT)
│   └── zk/           # Zero-knowledge proofs (gnark)
├── miner/quantum/    # Useful quantum mining module
├── p2p/              # P2P networking layer
├── rpc/              # JSON-RPC API implementation
└── ...               # Other go-ethereum modules

contracts/            # Solidity smart contracts
├── core/             # Core protocol contracts (QBTCToken, QPoARegistry)
├── pq/               # ZKnox post-quantum verifiers (FALCON, EPERVIER)
├── account/          # Hybrid account (EIP-7702, EIP-4337)
└── libraries/        # Solidity libraries (NTT, Math)

tests/                # Go and Solidity tests
├── unit/             # Unit tests for Go and Solidity
├── integration/      # Integration tests for the full node
├── benchmarks/       # Gas and performance benchmarks
└── vectors/          # NIST and ZKnox test vectors

.github/workflows/    # CI/CD pipeline (GitHub Actions)

docs/                 # Project documentation
├── whitepaper.md     # Institutional whitepaper
└── specs/            # Technical specifications

Dockerfile            # Multi-stage Dockerfile for the QBTC node
foundry.toml          # Foundry configuration for Solidity tests
```

## Post-Quantum Cryptography (ZKnox Integration)

QUBITCOIN integrates the state-of-the-art post-quantum cryptography developed by the **ZKnox** team, which is funded by the Ethereum Foundation to bring PQC to the Ethereum ecosystem.

| ZKnox Technology | Description | Gas Cost (on-chain) |
| :--- | :--- | :--- |
| **ETHFALCON** | EVM-optimized FALCON signature verification (keccak256). | **~1.5M gas** |
| **EPERVIER** | FALCON with address recovery (like `ecrecover`), enabling PQ account abstraction. | **~1.6M gas** |
| **ETHDilithium** | EVM-optimized ML-DSA signature verification. | ~4.9M gas |
| **NTT Library** | Number Theoretic Transform, the core building block for lattice-based crypto. | Pure Solidity fallback |
| **gnark Integration** | ZK-SNARKs for privacy, batch verification, and quantum-safe identity. | Groth16, PLONK |
| **PQBIP39** | Mnemonic generation for post-quantum keys, compatible with existing wallets. | N/A (off-chain) |

## Getting Started

### Prerequisites

- **Go**: version 1.22+
- **Foundry**: latest version
- **Docker**: latest version
- **Make**

### Build & Run from Source

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/NikaHsaini/Quantum-Blockchain-Pro.git
    cd Quantum-Blockchain-Pro
    ```

2.  **Install dependencies:**

    ```bash
    make install
    ```

3.  **Build the QBTC node:**

    ```bash
    cd qbtc-chain
    go build ./cmd/qbtc
    ```

4.  **Run a local dev node:**

    ```bash
    ./qbtc --dev --datadir /tmp/qbtc-data --http --ws --qpoa --quantum.mining
    ```

### Run with Docker

1.  **Build the Docker image:**

    ```bash
    docker build -t qubitcoin/qbtc-node:latest .
    ```

2.  **Run the Docker container:**

    ```bash
    docker run -p 8545:8545 -p 30303:30303 -v qbtc-data:/data/qbtc qubitcoin/qbtc-node:latest
    ```

## Testing & CI/CD

The project is equipped with a comprehensive CI/CD pipeline using GitHub Actions, which includes:

-   **Go Unit Tests & Benchmarks**: `go test -v -race ./...`
-   **Solidity Tests & Gas Benchmarks**: `forge test -vvv --gas-report`
-   **Security Audits**: `govulncheck`, `staticcheck`, `gosec`, `slither`
-   **PQC Validation**: Tests against NIST and ZKnox known-answer test (KAT) vectors.
-   **Integration Tests**: Full-node tests for consensus, account abstraction, and RPC.
-   **Docker Builds**: Automated Docker image builds.
-   **Automated Releases**: On git tag push.

To run tests locally:

```bash
# Run Go tests
cd qbtc-chain
go test -v ./...

# Run Solidity tests
forge test -vvv
```

## License

This project is licensed under the **MIT License**. See [LICENSE](LICENSE) for details.

## Disclaimer

QUBITCOIN is an experimental project. It is not audited and should not be used in production without a thorough security review. Use at your own risk.

---

*Authored by Nika Hsaini — QUBITCOIN Foundation*
