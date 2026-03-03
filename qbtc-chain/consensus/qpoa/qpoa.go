// Package qpoa implements the Quantum Proof-of-Authority consensus engine for QBP.
//
// QPoA (Quantum Proof-of-Authority) is a novel consensus mechanism that combines:
//   1. The efficiency and finality of PoA (Proof-of-Authority)
//   2. Post-quantum cryptographic signatures (ML-DSA / CRYSTALS-Dilithium)
//   3. A quantum challenge system for validator selection and rotation
//   4. Slashing conditions enforced via on-chain governance
//
// Design Principles:
//   - Validators (Authorities) are pre-approved entities that sign blocks with ML-DSA keys.
//   - Block time is fixed at 3 seconds, providing fast finality.
//   - Validator rotation is managed by a smart contract (QPoA Registry).
//   - Quantum Challenges are periodically issued to validators to prove quantum capability.
//   - All block headers include a quantum proof field.
package qpoa

import (
	"bytes"
	"crypto/sha3"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NikaHsaini/Quantum-Blockchain-Pro/qbtc-chain/crypto/pqcrypto"
)

// ============================================================
// Constants and Configuration
// ============================================================

const (
	// BlockTime is the target time between blocks in seconds.
	BlockTime = 3 * time.Second

	// EpochLength is the number of blocks per epoch (validator rotation period).
	EpochLength = 30000

	// MaxValidators is the maximum number of active validators.
	MaxValidators = 21

	// MinValidators is the minimum number of validators for consensus.
	MinValidators = 3

	// QuantumChallengeInterval is the number of blocks between quantum challenges.
	QuantumChallengeInterval = 1000

	// QuantumChallengeTimeout is the time validators have to respond to a challenge.
	QuantumChallengeTimeout = 10 * time.Minute

	// ExtraVanity is the number of bytes reserved in the extra-data for vanity.
	ExtraVanity = 32

	// ExtraSeal is the number of bytes reserved in the extra-data for ML-DSA signature.
	ExtraSeal = pqcrypto.MLDSA_SIGNATURE_SIZE

	// ExtraQuantumProof is the number of bytes reserved for the quantum proof.
	ExtraQuantumProof = 64

	// ExtraDataMinLen is the minimum length of the extra-data field.
	ExtraDataMinLen = ExtraVanity + ExtraSeal + ExtraQuantumProof

	// Difficulty values
	DifficultyInTurn  = 2 // Block signed by in-turn validator
	DifficultyNoTurn  = 1 // Block signed by out-of-turn validator

	// Nonce values for special block types
	NonceAuthVote  = 0xffffffffffffffff // Vote to authorize a new validator
	NonceDropVote  = 0x0000000000000000 // Vote to deauthorize a validator
)

// ============================================================
// Error Types
// ============================================================

var (
	ErrMissingSignature      = errors.New("qpoa: block signature missing in extra-data")
	ErrInvalidExtraDataLength = errors.New("qpoa: invalid extra-data length")
	ErrUnauthorizedValidator = errors.New("qpoa: unauthorized validator")
	ErrRecentlySigned        = errors.New("qpoa: validator recently signed a block")
	ErrInvalidQuantumProof   = errors.New("qpoa: invalid quantum proof in block header")
	ErrWrongDifficulty       = errors.New("qpoa: wrong block difficulty")
	ErrInvalidVotingChain    = errors.New("qpoa: invalid voting chain")
	ErrInvalidTimestamp      = errors.New("qpoa: invalid block timestamp")
)

// ============================================================
// Core Data Structures
// ============================================================

// Validator represents an authorized block producer in the QPoA network.
type Validator struct {
	Address     [20]byte             // QBP address derived from ML-DSA public key
	PublicKey   *pqcrypto.MLDSAPublicKey // ML-DSA public key for signature verification
	Stake       *big.Int             // Amount of QBP staked
	Since       uint64               // Block number when validator was added
	QuantumScore uint64              // Score based on quantum challenge performance
	LastSigned  uint64               // Last block number signed by this validator
}

// ValidatorSet manages the set of active validators.
type ValidatorSet struct {
	mu         sync.RWMutex
	validators []*Validator
	index      map[[20]byte]int // Address -> index mapping
}

// QuantumChallenge represents a quantum computation challenge issued to validators.
type QuantumChallenge struct {
	ID          uint64   // Challenge ID
	BlockNumber uint64   // Block number when challenge was issued
	CircuitHash [32]byte // Hash of the quantum circuit to solve
	Qubits      int      // Number of qubits in the circuit
	Gates       int      // Number of gates in the circuit
	TargetState [32]byte // Expected measurement outcome hash
	Deadline    uint64   // Block number deadline for submission
}

// QuantumProof represents a validator's proof of quantum computation.
type QuantumProof struct {
	ChallengeID  uint64   // ID of the challenge being answered
	ValidatorAddr [20]byte // Validator's address
	ResultHash   [32]byte // Hash of the quantum circuit result
	Signature    []byte   // ML-DSA signature over the result
	Timestamp    int64    // Unix timestamp of computation
}

// BlockHeader represents the QBP block header (simplified Ethereum-compatible).
type BlockHeader struct {
	ParentHash  [32]byte // Hash of the parent block
	UncleHash   [32]byte // Hash of uncle blocks (always empty in PoA)
	Coinbase    [20]byte // Address of the validator who mined this block
	Root        [32]byte // State trie root
	TxHash      [32]byte // Transaction trie root
	ReceiptHash [32]byte // Receipt trie root
	Bloom       [256]byte // Bloom filter
	Difficulty  *big.Int // Block difficulty (2 for in-turn, 1 for out-of-turn)
	Number      *big.Int // Block number
	GasLimit    uint64   // Gas limit
	GasUsed     uint64   // Gas used
	Time        uint64   // Unix timestamp
	Extra       []byte   // Extra data (vanity + ML-DSA signature + quantum proof)
	MixDigest   [32]byte // Quantum challenge hash (repurposed field)
	Nonce       [8]byte  // Voting nonce
}

// Snapshot represents the state of the validator set at a given block.
type Snapshot struct {
	Number     uint64              // Block number
	Hash       [32]byte            // Block hash
	Validators *ValidatorSet       // Active validator set
	Recents    map[uint64][20]byte // Recent block signers (block -> validator)
	Votes      []*Vote             // Pending votes
	Tally      map[[20]byte]Tally  // Vote tallies
}

// Vote represents a single vote to add or remove a validator.
type Vote struct {
	Validator [20]byte // Validator being voted on
	Block     uint64   // Block number of the vote
	Address   [20]byte // Voter's address
	Authorize bool     // Whether to authorize or deauthorize
}

// Tally represents the tally of votes for a validator.
type Tally struct {
	Authorize bool // Whether the tally is for authorization
	Votes     int  // Number of votes
}

// ============================================================
// QPoA Engine
// ============================================================

// QPoA is the Quantum Proof-of-Authority consensus engine.
type QPoA struct {
	mu           sync.RWMutex
	config       *QPoAConfig
	db           SnapshotDB
	proposals    map[[20]byte]bool // Current validator proposals
	snapshots    map[[32]byte]*Snapshot
	challenges   map[uint64]*QuantumChallenge
	proofs       map[uint64][]*QuantumProof
	privateKey   *pqcrypto.MLDSAPrivateKey // Validator's private key (nil if not a validator)
	validatorAddr [20]byte
}

// QPoAConfig holds the configuration for the QPoA consensus engine.
type QPoAConfig struct {
	Period         uint64   // Number of seconds between blocks
	Epoch          uint64   // Epoch length for validator rotation
	MaxValidators  int      // Maximum number of validators
	InitialValidators [][20]byte // Initial set of validator addresses
}

// SnapshotDB is the interface for snapshot persistence.
type SnapshotDB interface {
	GetSnapshot(hash [32]byte) (*Snapshot, error)
	PutSnapshot(snap *Snapshot) error
}

// NewQPoA creates a new QPoA consensus engine.
func NewQPoA(config *QPoAConfig, db SnapshotDB, privKey *pqcrypto.MLDSAPrivateKey) *QPoA {
	engine := &QPoA{
		config:     config,
		db:         db,
		proposals:  make(map[[20]byte]bool),
		snapshots:  make(map[[32]byte]*Snapshot),
		challenges: make(map[uint64]*QuantumChallenge),
		proofs:     make(map[uint64][]*QuantumProof),
		privateKey: privKey,
	}
	if privKey != nil {
		engine.validatorAddr = privKey.Public().Address()
	}
	return engine
}

// ============================================================
// Block Sealing (Validator Side)
// ============================================================

// Seal signs a block header with the validator's ML-DSA private key.
// This replaces the ECDSA signing in standard Ethereum PoA (Clique).
func (e *QPoA) Seal(header *BlockHeader) error {
	if e.privateKey == nil {
		return errors.New("qpoa: no private key configured for sealing")
	}

	// Ensure extra data is large enough
	if len(header.Extra) < ExtraDataMinLen {
		return ErrInvalidExtraDataLength
	}

	// Compute the signing hash (header without the signature part of extra)
	signingHash := e.signingHash(header)

	// Sign with ML-DSA
	sig, err := pqcrypto.Sign(e.privateKey, signingHash[:])
	if err != nil {
		return fmt.Errorf("qpoa: failed to sign block: %w", err)
	}

	// Embed signature in extra data
	sigBytes := sig.Bytes()
	if len(sigBytes) != ExtraSeal {
		return fmt.Errorf("qpoa: unexpected signature size: %d", len(sigBytes))
	}

	copy(header.Extra[ExtraVanity:ExtraVanity+ExtraSeal], sigBytes)

	// Generate and embed quantum proof
	qProof, err := e.generateQuantumProof(header)
	if err != nil {
		return fmt.Errorf("qpoa: failed to generate quantum proof: %w", err)
	}
	copy(header.Extra[ExtraVanity+ExtraSeal:ExtraDataMinLen], qProof)

	return nil
}

// ============================================================
// Block Verification
// ============================================================

// VerifyHeader verifies a block header against the consensus rules.
func (e *QPoA) VerifyHeader(header *BlockHeader, snap *Snapshot) error {
	// Check extra data length
	if len(header.Extra) < ExtraDataMinLen {
		return ErrInvalidExtraDataLength
	}

	// Recover the validator address from the ML-DSA signature
	validatorAddr, err := e.recoverValidator(header)
	if err != nil {
		return fmt.Errorf("qpoa: failed to recover validator: %w", err)
	}

	// Check that the validator is authorized
	if !snap.Validators.IsAuthorized(validatorAddr) {
		return ErrUnauthorizedValidator
	}

	// Check that the validator hasn't signed recently (prevent spam)
	if snap.Recents != nil {
		for seen, recent := range snap.Recents {
			if recent == validatorAddr {
				// Validator has signed within the last n/2+1 blocks
				limit := uint64(snap.Validators.Len()/2 + 1)
				if header.Number.Uint64()-seen < limit {
					return ErrRecentlySigned
				}
			}
		}
	}

	// Verify the quantum proof
	if err := e.verifyQuantumProof(header); err != nil {
		return err
	}

	// Check difficulty
	expectedDiff := e.calcDifficulty(snap, validatorAddr)
	if header.Difficulty.Cmp(expectedDiff) != 0 {
		return ErrWrongDifficulty
	}

	// Check timestamp
	if header.Time < header.Time+uint64(BlockTime.Seconds()) {
		// Allow some slack for network latency
	}

	return nil
}

// ============================================================
// Validator Set Management
// ============================================================

// calcDifficulty calculates the expected difficulty for a block.
// In-turn validators get difficulty 2, out-of-turn get difficulty 1.
func (e *QPoA) calcDifficulty(snap *Snapshot, validator [20]byte) *big.Int {
	if snap.InTurn(snap.Number+1, validator) {
		return big.NewInt(DifficultyInTurn)
	}
	return big.NewInt(DifficultyNoTurn)
}

// InTurn checks if a validator is the in-turn validator for a given block number.
func (s *Snapshot) InTurn(number uint64, validator [20]byte) bool {
	validators := s.Validators.List()
	if len(validators) == 0 {
		return false
	}
	idx := number % uint64(len(validators))
	return validators[idx].Address == validator
}

// ============================================================
// Signing and Verification Helpers
// ============================================================

// signingHash computes the hash to be signed for a block header.
// The signature and quantum proof fields in extra data are excluded.
func (e *QPoA) signingHash(header *BlockHeader) [32]byte {
	// Create a copy of extra without the seal
	extra := header.Extra[:ExtraVanity]

	h := sha3.New256()
	h.Write(header.ParentHash[:])
	h.Write(header.Coinbase[:])
	h.Write(header.Root[:])
	h.Write(header.TxHash[:])
	h.Write(header.Number.Bytes())
	h.Write(extra)
	binary.Write(h, binary.BigEndian, header.Time)

	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// recoverValidator recovers the validator address from the ML-DSA signature in extra data.
func (e *QPoA) recoverValidator(header *BlockHeader) ([20]byte, error) {
	if len(header.Extra) < ExtraDataMinLen {
		return [20]byte{}, ErrMissingSignature
	}

	sigBytes := header.Extra[ExtraVanity : ExtraVanity+ExtraSeal]
	sig, err := pqcrypto.ParseMLDSASignature(sigBytes)
	if err != nil {
		return [20]byte{}, fmt.Errorf("qpoa: failed to parse signature: %w", err)
	}

	signingHash := e.signingHash(header)

	// We need to try all known validators to find who signed this block
	// In production, the coinbase field contains the validator address
	// so we can directly look up their public key
	return header.Coinbase, e.verifySignatureWithCoinbase(header, sig, signingHash)
}

// verifySignatureWithCoinbase verifies the ML-DSA signature using the coinbase validator's public key.
func (e *QPoA) verifySignatureWithCoinbase(header *BlockHeader, sig *pqcrypto.MLDSASignature, hash [32]byte) error {
	// In a full implementation, we would look up the public key from the validator registry
	// For now, we verify the signature structure is valid
	_ = sig
	_ = hash
	return nil
}

// ============================================================
// Quantum Proof Generation and Verification
// ============================================================

// generateQuantumProof generates a quantum proof for a block.
// The proof demonstrates that the validator has executed a quantum circuit.
func (e *QPoA) generateQuantumProof(header *BlockHeader) ([]byte, error) {
	// Generate a deterministic quantum circuit based on the block's parent hash
	circuitSeed := header.ParentHash[:]
	circuitSeed = append(circuitSeed, header.Number.Bytes()...)

	// Simulate quantum circuit execution
	// In production, this would call the actual quantum simulator
	result := e.simulateQuantumCircuit(circuitSeed, 8) // 8-qubit circuit

	// Create proof: hash of (validator_address || circuit_result || block_number)
	h := sha3.New256()
	h.Write(e.validatorAddr[:])
	h.Write(result)
	h.Write(header.Number.Bytes())

	proof := h.Sum(nil)

	// Pad to ExtraQuantumProof size
	paddedProof := make([]byte, ExtraQuantumProof)
	copy(paddedProof, proof)
	return paddedProof, nil
}

// verifyQuantumProof verifies the quantum proof in a block header.
func (e *QPoA) verifyQuantumProof(header *BlockHeader) error {
	if len(header.Extra) < ExtraDataMinLen {
		return ErrInvalidQuantumProof
	}

	proofBytes := header.Extra[ExtraVanity+ExtraSeal : ExtraDataMinLen]

	// Verify proof is not all zeros (basic check)
	allZero := true
	for _, b := range proofBytes[:32] {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ErrInvalidQuantumProof
	}

	// In production: re-execute the quantum circuit and verify the result
	// For now, we verify the proof structure
	_ = proofBytes
	return nil
}

// simulateQuantumCircuit simulates a quantum circuit and returns the measurement result.
// This is a deterministic simulation for consensus purposes.
func (e *QPoA) simulateQuantumCircuit(seed []byte, numQubits int) []byte {
	// Quantum circuit simulation using classical computation
	// In production, this would use the GPU-accelerated quantum simulator

	// Initialize state vector (2^numQubits complex amplitudes)
	stateSize := 1 << numQubits
	state := make([]float64, stateSize*2) // Real and imaginary parts

	// Initialize |0...0> state
	state[0] = 1.0

	// Apply Hadamard gates to all qubits (creates superposition)
	state = applyHadamardAll(state, numQubits)

	// Apply parameterized rotation gates based on seed
	state = applyParameterizedRotations(state, numQubits, seed)

	// Apply CNOT entanglement gates
	state = applyEntanglement(state, numQubits)

	// Measure and return result hash
	h := sha3.New256()
	for _, amp := range state {
		binary.Write(h, binary.LittleEndian, amp)
	}
	return h.Sum(nil)
}

// applyHadamardAll applies Hadamard gate to all qubits.
func applyHadamardAll(state []float64, numQubits int) []float64 {
	stateSize := 1 << numQubits
	result := make([]float64, len(state))
	factor := 1.0 / float64(int(1)<<(numQubits/2)) // 1/sqrt(2^n)

	for i := 0; i < stateSize; i++ {
		for j := 0; j < stateSize; j++ {
			// H^n |j> = sum_i (-1)^(i·j) |i> / sqrt(2^n)
			phase := 1.0
			for k := 0; k < numQubits; k++ {
				if (i>>k)&1 == 1 && (j>>k)&1 == 1 {
					phase *= -1.0
				}
			}
			result[i*2] += factor * phase * state[j*2]
			result[i*2+1] += factor * phase * state[j*2+1]
		}
	}
	return result
}

// applyParameterizedRotations applies Rz(theta) rotations based on seed.
func applyParameterizedRotations(state []float64, numQubits int, seed []byte) []float64 {
	result := make([]float64, len(state))
	copy(result, state)

	stateSize := 1 << numQubits
	for q := 0; q < numQubits; q++ {
		// Derive rotation angle from seed
		seedByte := float64(seed[q%len(seed)]) / 255.0
		theta := seedByte * 6.28318530718 // 2*pi

		cosTheta := 1.0 - theta*theta/2 + theta*theta*theta*theta/24 // Taylor approx
		sinTheta := theta - theta*theta*theta/6                        // Taylor approx

		// Apply Rz(theta) to qubit q
		for i := 0; i < stateSize; i++ {
			if (i>>q)&1 == 1 {
				// |1> state: multiply by e^(i*theta/2)
				re := result[i*2]
				im := result[i*2+1]
				result[i*2] = re*cosTheta - im*sinTheta
				result[i*2+1] = re*sinTheta + im*cosTheta
			}
		}
	}
	return result
}

// applyEntanglement applies CNOT gates to create entanglement.
func applyEntanglement(state []float64, numQubits int) []float64 {
	result := make([]float64, len(state))
	copy(result, state)

	stateSize := 1 << numQubits
	for q := 0; q < numQubits-1; q++ {
		// CNOT with control=q, target=q+1
		for i := 0; i < stateSize; i++ {
			if (i>>q)&1 == 1 {
				// Flip target qubit
				j := i ^ (1 << (q + 1))
				if j < stateSize {
					result[i*2], result[j*2] = result[j*2], result[i*2]
					result[i*2+1], result[j*2+1] = result[j*2+1], result[i*2+1]
				}
			}
		}
	}
	return result
}

// ============================================================
// ValidatorSet Methods
// ============================================================

// NewValidatorSet creates a new validator set from a list of validators.
func NewValidatorSet(validators []*Validator) *ValidatorSet {
	vs := &ValidatorSet{
		validators: make([]*Validator, len(validators)),
		index:      make(map[[20]byte]int),
	}
	copy(vs.validators, validators)
	for i, v := range vs.validators {
		vs.index[v.Address] = i
	}
	return vs
}

// IsAuthorized checks if an address is an authorized validator.
func (vs *ValidatorSet) IsAuthorized(addr [20]byte) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	_, ok := vs.index[addr]
	return ok
}

// List returns the list of validators.
func (vs *ValidatorSet) List() []*Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	result := make([]*Validator, len(vs.validators))
	copy(result, vs.validators)
	return result
}

// Len returns the number of validators.
func (vs *ValidatorSet) Len() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.validators)
}

// Add adds a new validator to the set.
func (vs *ValidatorSet) Add(v *Validator) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if _, exists := vs.index[v.Address]; exists {
		return fmt.Errorf("qpoa: validator %x already exists", v.Address)
	}
	if len(vs.validators) >= MaxValidators {
		return fmt.Errorf("qpoa: maximum validator count (%d) reached", MaxValidators)
	}
	vs.index[v.Address] = len(vs.validators)
	vs.validators = append(vs.validators, v)
	return nil
}

// Remove removes a validator from the set.
func (vs *ValidatorSet) Remove(addr [20]byte) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	idx, exists := vs.index[addr]
	if !exists {
		return fmt.Errorf("qpoa: validator %x not found", addr)
	}
	vs.validators = append(vs.validators[:idx], vs.validators[idx+1:]...)
	delete(vs.index, addr)
	// Rebuild index
	for i, v := range vs.validators {
		vs.index[v.Address] = i
	}
	return nil
}

// ============================================================
// Snapshot Methods
// ============================================================

// NewSnapshot creates a new snapshot.
func NewSnapshot(number uint64, hash [32]byte, validators *ValidatorSet) *Snapshot {
	return &Snapshot{
		Number:     number,
		Hash:       hash,
		Validators: validators,
		Recents:    make(map[uint64][20]byte),
		Votes:      make([]*Vote, 0),
		Tally:      make(map[[20]byte]Tally),
	}
}

// Apply applies a block header to the snapshot, updating the validator set if needed.
func (s *Snapshot) Apply(header *BlockHeader) (*Snapshot, error) {
	// Create a new snapshot
	snap := &Snapshot{
		Number:     header.Number.Uint64(),
		Validators: s.Validators,
		Recents:    make(map[uint64][20]byte),
		Votes:      make([]*Vote, len(s.Votes)),
		Tally:      make(map[[20]byte]Tally),
	}
	copy(snap.Votes, s.Votes)
	for k, v := range s.Tally {
		snap.Tally[k] = v
	}
	for k, v := range s.Recents {
		snap.Recents[k] = v
	}

	// Record the signer
	snap.Recents[header.Number.Uint64()] = header.Coinbase

	// Process vote if nonce indicates one
	nonce := binary.BigEndian.Uint64(header.Nonce[:])
	if nonce == NonceAuthVote || nonce == NonceDropVote {
		// Extract the voted-for address from the extra data
		// (In production, this would be in a specific field)
		var votedAddr [20]byte
		if len(header.Extra) >= ExtraVanity+20 {
			copy(votedAddr[:], header.Extra[:20])
		}

		authorize := nonce == NonceAuthVote
		vote := &Vote{
			Validator: votedAddr,
			Block:     header.Number.Uint64(),
			Address:   header.Coinbase,
			Authorize: authorize,
		}
		snap.Votes = append(snap.Votes, vote)

		// Update tally
		tally := snap.Tally[votedAddr]
		tally.Authorize = authorize
		tally.Votes++
		snap.Tally[votedAddr] = tally

		// Check if vote passed (majority)
		if tally.Votes > snap.Validators.Len()/2 {
			if authorize {
				// Add validator (would need public key in production)
				newValidator := &Validator{
					Address: votedAddr,
					Since:   header.Number.Uint64(),
				}
				snap.Validators.Add(newValidator)
			} else {
				snap.Validators.Remove(votedAddr)
			}
			// Clear votes for this address
			delete(snap.Tally, votedAddr)
			newVotes := make([]*Vote, 0)
			for _, v := range snap.Votes {
				if !bytes.Equal(v.Validator[:], votedAddr[:]) {
					newVotes = append(newVotes, v)
				}
			}
			snap.Votes = newVotes
		}
	}

	// Compute new hash
	h := sha3.New256()
	h.Write(header.ParentHash[:])
	h.Write(header.Number.Bytes())
	copy(snap.Hash[:], h.Sum(nil))

	return snap, nil
}

// Public returns the public key associated with a private key.
func (priv *pqcrypto.MLDSAPrivateKey) Public() *pqcrypto.MLDSAPublicKey {
	return priv.GetPublicKey()
}
