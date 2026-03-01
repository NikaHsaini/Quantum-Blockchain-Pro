# QUBITCOIN (QBTC)

### Post-Quantum Financial Infrastructure for European Digital Sovereignty

> *"The first blockchain protocol natively secured against quantum computers, compliant with European regulations, and designed for institutional-grade digital finance."*
>
> — **Nika Hsaini**, Founder

---

## Overview

QUBITCOIN is a third-generation blockchain protocol that forks and radically extends the Ethereum codebase (`go-ethereum`) with a comprehensive stack of post-quantum cryptographic technologies, a novel consensus mechanism, and a decentralized quantum computing marketplace. It is designed to be the foundational infrastructure for the European digital economy in the post-quantum era.

| Property | Value |
| :--- | :--- |
| **Token Symbol** | QBTC |
| **Total Supply** | 21,000 QBTC (permanently fixed, no inflation) |
| **Initial Price Target** | €100,000 per QBTC |
| **Total Network Valuation** | €2.1 Billion |
| **Consensus** | Quantum Proof-of-Authority (QPoA) |
| **EVM Compatibility** | Full (+ 16 Quantum Opcodes) |
| **Primary Signature Scheme** | FALCON-1024 (NIST FIPS 206) |
| **Secondary Signature Scheme** | ML-DSA-65 / CRYSTALS-Dilithium (NIST FIPS 204) |
| **Key Encapsulation** | ML-KEM / CRYSTALS-Kyber (NIST FIPS 203) |
| **Hash Function** | SHA-999 (Triple-layer SHA3-512, quantum-resistant) |
| **Regulatory Compliance** | MiCA · eIDAS 2.0 · DORA |
| **Governance** | QUBITCOIN Foundation (Switzerland) |
| **Language** | Go (core protocol) · Solidity (smart contracts) |

---

## The Quantum Threat

Shor's algorithm, running on a sufficiently powerful quantum computer, will break ECDSA — the signature scheme securing Bitcoin, Ethereum, and virtually every existing blockchain. The "store now, decrypt later" attack vector means that transactions signed today can be decrypted by future quantum computers. QUBITCOIN is the definitive answer to this existential threat.

---

## Three Foundational Pillars

### 1. Post-Quantum Security (Crypto-Agile Architecture)

QUBITCOIN implements the final NIST post-quantum standards, making it the most secure blockchain protocol available today.

| Algorithm | Standard | Role | Key Size |
| :--- | :--- | :--- | :--- |
| **FALCON-1024** | NIST FIPS 206 | Primary digital signature | PK: 1793 bytes, Sig: ~1330 bytes |
| **ML-DSA-65** | NIST FIPS 204 | Secondary / fallback signature | PK: 1952 bytes |
| **ML-KEM-1024** | NIST FIPS 203 | Key encapsulation (P2P comms) | PK: 1568 bytes |
| **SHA-999** | QBTC Native | Quantum-resistant block hash | 512-bit output |

The **crypto-agile architecture** allows the protocol to migrate to new cryptographic algorithms without network disruption, ensuring long-term security as the quantum threat landscape evolves.

### 2. Regulatory Compliance ("Compliant-by-Design")

QUBITCOIN is the only blockchain protocol designed from the ground up to comply with the full European regulatory framework:

- **MiCA (Markets in Crypto-Assets)**: QBTC is structured as a pure utility token. The network includes CASP-ready features for compliant exchange operations and AML/CFT blacklisting.
- **eIDAS 2.0**: On-chain binding of European Digital Identity Wallets enables seamless, privacy-preserving KYC/AML for institutional and retail users.
- **DORA (Digital Operational Resilience Act)**: The PQC infrastructure directly helps financial institutions comply with quantum cyber-risk management obligations.

### 3. European Digital Sovereignty

All validator nodes are hosted within EU member states. Governance is managed by a neutral Swiss Foundation. The network is powered by green energy through partnerships with European energy providers, aligning with the EU's sustainability objectives.

---

## Repository Structure

```
Quantum-Blockchain-Pro/
├── qbp-chain/                          # Core blockchain implementation (Go)
│   ├── cmd/qbp/main.go                 # Node CLI entry point
│   ├── consensus/qpoa/qpoa.go          # Quantum Proof-of-Authority consensus
│   ├── core/vm/quantum/qevm.go         # Quantum-enhanced EVM (16 new opcodes)
│   ├── crypto/pqcrypto/
│   │   ├── falcon.go                   # FALCON-1024 (primary PQ signature)
│   │   ├── mldsa.go                    # ML-DSA-65 / CRYSTALS-Dilithium
│   │   ├── mlkem.go                    # ML-KEM / CRYSTALS-Kyber
│   │   ├── keys.go                     # PQ account management & address derivation
│   │   └── sha999.go                   # SHA-999 quantum-resistant hash (in falcon.go)
│   ├── miner/quantum/qminer.go         # Quantum Mining as a Service (QMaaS)
│   ├── sdk/
│   │   ├── go/qbp.go                   # Go SDK
│   │   └── js/qbp.js                   # JavaScript SDK (ethers.js compatible)
│   └── go.mod
├── contracts/quantum/                  # Solidity smart contracts
│   ├── QBTCToken.sol                   # QBTC ERC-20 (FALCON + ML-DSA + vesting + MiCA)
│   ├── QPoARegistry.sol                # Validator registry, staking & slashing
│   └── QuantumOracle.sol               # On-chain quantum computation oracle (QMaaS)
└── docs/
    └── whitepaper.md                   # Full institutional whitepaper v3.0
```

---

## QBTC Token

```
Token Name:     QUBITCOIN
Token Symbol:   QBTC
Total Supply:   21,000 QBTC (permanently fixed, no inflation possible)
Decimals:       18
Standard:       ERC-20 + Post-Quantum Extensions
Contract:       contracts/quantum/QBTCToken.sol
```

### Token Allocation

| Category | % | QBTC | Vesting |
| :--- | :--- | :--- | :--- |
| Protocol & Ecosystem Development | 30% | 6,300 | Foundation managed, long-term |
| Ecosystem & Staking Rewards | 25% | 5,250 | Distributed over network lifetime |
| Strategic Investors (Seed & Private) | 20% | 4,200 | 4-year vesting |
| Founding Team & Advisors | 15% | 3,150 | 4-year vesting, 1-year cliff |
| Public Sale & Liquidity | 10% | 2,100 | Available on regulated EU exchanges |

### Valuation Justification: €100,000 per QBTC

The initial price target of €100,000 per QBTC is grounded in five converging factors:

1.  **Extreme Scarcity**: 21,000 QBTC total supply — 1,000x rarer than Bitcoin. A structural foundation for long-term value.
2.  **Quantum Security Premium**: The only blockchain resistant to Shor's algorithm. As quantum computers advance, this security premium will increase exponentially.
3.  **Intrinsic Utility**: QBTC is the fuel for a global quantum computing marketplace (QMaaS), a B2B SaaS platform, and a regulated financial infrastructure.
4.  **Institutional Demand**: Designed for financial institutions, governments, and enterprises requiring the highest levels of security and regulatory compliance.
5.  **10-Year Valuation Trajectory**: Based on DCF analysis and Metcalfe's Law, the QUBITCOIN ecosystem is projected to exceed **€22 billion in total valuation by 2036**.

---

## Quantum Technology Stack

### FALCON-1024 (Primary Signature Scheme)

```go
// Generate a FALCON-1024 key pair
keyPair, err := pqcrypto.GenerateFALCONKeyPair()

// Sign a transaction
signature, err := keyPair.PrivateKey.Sign(txHash)

// Verify a signature
valid := keyPair.PublicKey.Verify(txHash, signature)
```

### SHA-999 (Quantum-Resistant Hash)

```
SHA-999(x) = SHA3-512("QBTC-SHA999-R3" || SHA3-512("QBTC-SHA999-R2" || SHA3-512("QBTC-SHA999-R1" || x)))
```

### Crypto-Agile Migration

```go
// Sign with any supported PQC algorithm
sig, err := pqcrypto.CryptoAgileSign(message, "FALCON-1024", privateKeyBytes)

// Verify with any supported PQC algorithm
valid, err := pqcrypto.CryptoAgileVerify(message, sig, publicKeyBytes, "FALCON-1024")
```

### Quantum Opcodes (qEVM)

| Opcode | Name | Description |
| :--- | :--- | :--- |
| `0xE0` | `QGROVER` | Grover's quantum search algorithm |
| `0xE1` | `QQFT` | Quantum Fourier Transform |
| `0xE2` | `QVQE` | Variational Quantum Eigensolver |
| `0xE3` | `QENTANGLE` | Quantum entanglement circuit |
| `0xE4` | `QVERIFYPQ` | Verify post-quantum signature on-chain |
| `0xE5` | `QRANDOM` | Quantum random number generation |
| `0xE6` | `QSHOR` | Shor's factoring algorithm |
| `0xE7` | `QPHASE` | Quantum phase estimation |
| `0xE8` | `QAMPLIFY` | Amplitude amplification |
| `0xE9` | `QSWAP` | Quantum SWAP test |
| `0xEA` | `QANNEALING` | Quantum annealing optimization |
| `0xEB` | `QKEYEX` | ML-KEM key exchange |
| `0xEC` | `QHASH999` | SHA-999 on-chain computation |
| `0xED` | `QMLDSA` | ML-DSA signature verification |
| `0xEE` | `QFALCON` | FALCON signature verification |
| `0xEF` | `QCIRCUIT` | Execute arbitrary quantum circuit |

---

## 5-Year Financial Projections

| Indicator (€M) | Year 1 | Year 2 | Year 3 | Year 4 | Year 5 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Total Revenue | 2.0 | 8.5 | 28.0 | 48.0 | 75.0 |
| EBITDA | (4.5) | 0.5 | 16.0 | 33.0 | 57.0 |
| Net Income | (5.5) | (0.5) | 13.0 | 28.0 | 48.0 |

**Break-even: Year 2. Seed funding target: €5-7M.**

---

## Roadmap

| Phase | Timeline | Key Milestones |
| :--- | :--- | :--- |
| **Phase 1: Foundation & Seeding** | 2026-2027 | Mainnet launch, 5 institutional validators (QPoA), MiCA license, first 5 B2B contracts |
| **Phase 2: Growth & Expansion** | 2028-2030 | 100 validators, €1B annual transaction volume, profitability, Series A (€15-25M) |
| **Phase 3: Mass Adoption & Sovereignty** | 2031-2036 | Digital Euro integration, €100B annual volume, potential IPO, Series B+ |

---

## Getting Started

```bash
# Clone the repository
git clone https://github.com/NikaHsaini/Quantum-Blockchain-Pro.git
cd Quantum-Blockchain-Pro/qbp-chain

# Build the QBTC node
go build -o qbtc ./cmd/qbp/

# Initialize a new node
./qbtc init --datadir ~/.qbtc

# Start the node (mainnet)
./qbtc start --datadir ~/.qbtc --network mainnet

# Create a new post-quantum account (FALCON-1024)
./qbtc account new --algorithm FALCON-1024
```

---

## License

This project is licensed under the **GNU Lesser General Public License v3.0 (LGPL-3.0)**, consistent with the `go-ethereum` codebase from which it is derived.

---

> **Full Institutional Whitepaper**: See [`docs/whitepaper.md`](docs/whitepaper.md) for the complete technical and economic analysis.

---

*Built with precision and ambition by **Nika Hsaini** — QUBITCOIN Foundation, 2026.*
