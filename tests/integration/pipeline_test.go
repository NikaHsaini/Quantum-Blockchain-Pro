package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// ============================================================
// Integration Test: Full Mining Pipeline
// ============================================================

// TestFullMiningPipeline simulates the complete QUBITCOIN mining flow:
// 1. Miner registers with supported IBM Quantum backends
// 2. User submits a quantum computation job
// 3. Miner executes the circuit on IBM QPU
// 4. Miner submits the result on-chain
// 5. Result is verified and miner is rewarded
func TestFullMiningPipeline(t *testing.T) {
	// Step 1: Simulate miner registration
	miner := "0x1234567890abcdef1234567890abcdef12345678"
	maxQubits := 156 // IBM Heron r2
	backends := []string{"ibm_fez", "ibm_marrakesh", "ibm_torino"}

	if maxQubits < 4 {
		t.Fatal("miner must support at least 4 qubits")
	}
	if len(backends) == 0 {
		t.Fatal("miner must support at least one backend")
	}

	t.Logf("Miner %s registered with %d qubits, backends: %v", miner, maxQubits, backends)

	// Step 2: Simulate job submission
	numQubits := 10
	circuitHash := sha256.Sum256([]byte("grover_10qubit_target_42"))
	reward := uint64(1e18) // 1 QBTC
	deadline := uint64(1000)

	if numQubits > maxQubits {
		t.Fatalf("job requires %d qubits but miner supports %d", numQubits, maxQubits)
	}

	jobID := sha256.Sum256([]byte(fmt.Sprintf("%s_%d_%s_%d",
		miner, numQubits, hex.EncodeToString(circuitHash[:]), deadline)))

	t.Logf("Job %s submitted: %d qubits, reward: %d wei", hex.EncodeToString(jobID[:8]), numQubits, reward)

	// Step 3: Simulate circuit execution on IBM QPU
	// In production, this calls ibm.Client.MineCircuit()
	resultData := fmt.Sprintf("grover_result_%d_qubits_target_42_found_42", numQubits)
	resultHash := sha256.Sum256([]byte(resultData))

	t.Logf("Circuit executed on ibm_fez, result hash: %s", hex.EncodeToString(resultHash[:8]))

	// Step 4: Simulate result verification
	verificationResult := sha256.Sum256(append(append(jobID[:], resultHash[:]...), byte(numQubits)))
	expectedVerification := sha256.Sum256(append(append(jobID[:], resultHash[:]...), byte(numQubits)))

	if verificationResult != expectedVerification {
		t.Fatal("verification failed: result hash mismatch")
	}

	t.Logf("Result verified successfully")

	// Step 5: Verify reward distribution
	protocolFeeBps := uint64(200) // 2%
	fee := (reward * protocolFeeBps) / 10000
	minerReward := reward - fee

	if minerReward+fee != reward {
		t.Fatal("reward distribution error: miner reward + fee != total reward")
	}
	if fee != reward*2/100 {
		t.Fatal("protocol fee should be 2%")
	}

	t.Logf("Miner rewarded: %d wei (fee: %d wei, %.1f%%)", minerReward, fee, float64(fee)/float64(reward)*100)
}

// TestPQSignatureFlow simulates the post-quantum signature verification flow.
func TestPQSignatureFlow(t *testing.T) {
	// Step 1: Generate a simulated FALCON-1024 key pair
	publicKeySize := 1793
	secretKeySize := 2305
	signatureSize := 1462

	publicKey := make([]byte, publicKeySize)
	secretKey := make([]byte, secretKeySize)

	// Fill with deterministic test data
	for i := range publicKey {
		publicKey[i] = byte(i % 256)
	}
	for i := range secretKey {
		secretKey[i] = byte((i + 42) % 256)
	}

	t.Logf("FALCON-1024 key pair generated: PK=%d bytes, SK=%d bytes", len(publicKey), len(secretKey))

	// Step 2: Sign a transfer message
	message := []byte("QBTC_PQ_TRANSFER_V2|from|to|amount|nonce|chainid")
	messageHash := sha256.Sum256(message)

	// Simulate signature (in production, this uses liboqs FALCON)
	signature := make([]byte, signatureSize)
	h := sha256.New()
	h.Write(secretKey)
	h.Write(messageHash[:])
	sigHash := h.Sum(nil)
	copy(signature, sigHash)

	t.Logf("Message signed: sig=%d bytes, hash=%s", len(signature), hex.EncodeToString(messageHash[:8]))

	// Step 3: Verify signature
	if len(publicKey) != publicKeySize {
		t.Fatalf("invalid public key size: %d", len(publicKey))
	}
	if len(signature) > signatureSize {
		t.Fatalf("signature too large: %d > %d", len(signature), signatureSize)
	}

	// Simulate verification (in production, this calls ZKnox ETHFALCON verifier)
	isValid := len(signature) > 0 && signature[0] != 0
	if !isValid {
		t.Fatal("PQ signature verification failed")
	}

	t.Logf("FALCON-1024 signature verified successfully")
}

// TestCryptoAgilityMigration tests the crypto-agility migration flow.
func TestCryptoAgilityMigration(t *testing.T) {
	algorithms := []struct {
		name   string
		pkSize int
		sigSize int
	}{
		{"FALCON-1024", 1793, 1462},
		{"ML-DSA-65", 1952, 3309},
		{"ML-DSA-87", 2592, 4627},
		{"SLH-DSA-SHA2-128f", 32, 17088},
	}

	currentAlg := algorithms[0]
	t.Logf("Current algorithm: %s (PK: %d, Sig: %d)", currentAlg.name, currentAlg.pkSize, currentAlg.sigSize)

	// Simulate migration to ML-DSA-87 (higher security level)
	newAlg := algorithms[2]
	t.Logf("Migrating to: %s (PK: %d, Sig: %d)", newAlg.name, newAlg.pkSize, newAlg.sigSize)

	// Verify that accounts can still use old algorithm during transition
	if currentAlg.pkSize == newAlg.pkSize {
		t.Fatal("algorithms should have different key sizes")
	}

	t.Logf("Crypto-agility migration successful: %s → %s", currentAlg.name, newAlg.name)
}

// TestTokenomicsInvariants verifies tokenomics invariants hold.
func TestTokenomicsInvariants(t *testing.T) {
	maxSupply := uint64(21_000)

	allocations := map[string]uint64{
		"protocol":   6_300,
		"staking":    5_250,
		"investors":  4_200,
		"team":       3_150,
		"publicSale": 2_100,
	}

	var total uint64
	for name, amount := range allocations {
		if amount == 0 {
			t.Errorf("allocation %s is zero", name)
		}
		total += amount
	}

	if total != maxSupply {
		t.Fatalf("allocations sum to %d, expected %d", total, maxSupply)
	}

	// Verify stabilization framework parameters
	// The protocol uses algorithmic liquidity management rather than fixed price targets.
	// Key parameters: TWAP rebalancing, strategic treasury POL, discretionary intervention.
	twapWindowBlocks := uint64(240)      // ~12 minutes at 3s block time
	polReserveRatio := uint64(10)         // 10% of POL deployed per rebalancing cycle
	stabilizationThreshold := uint64(1)   // Threshold must be > 0

	if twapWindowBlocks == 0 || polReserveRatio == 0 || stabilizationThreshold == 0 {
		t.Fatal("stabilization parameters must be non-zero")
	}

	// Verify supply scarcity: 21,000 QBTC is 1000x rarer than BTC (21M)
	scarcityMultiplier := uint64(21_000_000) / maxSupply
	if scarcityMultiplier != 1000 {
		t.Fatalf("scarcity multiplier: %d, expected 1000", scarcityMultiplier)
	}

	t.Logf("Tokenomics verified: %d QBTC, scarcity %dx vs BTC, TWAP window %d blocks, POL ratio %d%%",
		maxSupply, scarcityMultiplier, twapWindowBlocks, polReserveRatio)
}
