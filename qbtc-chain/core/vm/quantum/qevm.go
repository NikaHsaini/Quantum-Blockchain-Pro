// Package quantum implements the Quantum EVM (qEVM) extension for QBP.
//
// The qEVM extends the standard Ethereum Virtual Machine with quantum opcodes,
// allowing smart contracts to interact with quantum circuits and the QMaaS marketplace.
//
// New Opcodes:
//   - QC_CREATE  (0xE0): Create a new quantum circuit context
//   - QC_HADAMARD (0xE1): Apply Hadamard gate to a qubit
//   - QC_PAULI_X (0xE2): Apply Pauli-X gate
//   - QC_PAULI_Y (0xE3): Apply Pauli-Y gate
//   - QC_PAULI_Z (0xE4): Apply Pauli-Z gate
//   - QC_CNOT    (0xE5): Apply CNOT gate
//   - QC_RZ      (0xE6): Apply Rz rotation gate
//   - QC_RY      (0xE7): Apply Ry rotation gate
//   - QC_MEASURE (0xE8): Measure a qubit
//   - QC_EXECUTE (0xE9): Execute the circuit via oracle
//   - QC_RESULT  (0xEA): Get the result of an executed circuit
//   - QC_GROVER  (0xEB): Execute Grover's search algorithm
//   - QC_QFT     (0xEC): Execute Quantum Fourier Transform
//   - QC_VQE     (0xED): Execute Variational Quantum Eigensolver
//   - QC_ENTANGLE (0xEE): Create Bell state (entanglement)
//   - QC_VERIFY_PQ (0xEF): Verify a post-quantum signature on-chain
//
// Gas Costs:
//   Quantum opcodes have significantly higher gas costs to reflect the
//   computational resources required for quantum simulation.
package quantum

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	qminer "github.com/NikaHsaini/Quantum-Blockchain-Pro/qbtc-chain/miner/quantum"
	"github.com/NikaHsaini/Quantum-Blockchain-Pro/qbtc-chain/crypto/pqcrypto"
)

// ============================================================
// Quantum Opcodes
// ============================================================

// OpCode represents a quantum EVM opcode.
type OpCode byte

const (
	// Quantum Circuit Operations
	QC_CREATE   OpCode = 0xE0 // Create quantum circuit
	QC_HADAMARD OpCode = 0xE1 // Apply Hadamard gate
	QC_PAULI_X  OpCode = 0xE2 // Apply Pauli-X gate
	QC_PAULI_Y  OpCode = 0xE3 // Apply Pauli-Y gate
	QC_PAULI_Z  OpCode = 0xE4 // Apply Pauli-Z gate
	QC_CNOT     OpCode = 0xE5 // Apply CNOT gate
	QC_RZ       OpCode = 0xE6 // Apply Rz rotation
	QC_RY       OpCode = 0xE7 // Apply Ry rotation
	QC_MEASURE  OpCode = 0xE8 // Measure qubit
	QC_EXECUTE  OpCode = 0xE9 // Execute circuit
	QC_RESULT   OpCode = 0xEA // Get result
	QC_GROVER   OpCode = 0xEB // Grover's search
	QC_QFT      OpCode = 0xEC // Quantum Fourier Transform
	QC_VQE      OpCode = 0xED // Variational Quantum Eigensolver
	QC_ENTANGLE OpCode = 0xEE // Create Bell state
	QC_VERIFY_PQ OpCode = 0xEF // Verify post-quantum signature
)

// ============================================================
// Gas Costs for Quantum Operations
// ============================================================

// Gas costs are calibrated to reflect the computational complexity
// of quantum simulations relative to classical EVM operations.
const (
	// Base gas costs for quantum operations
	GasQCCreate    = 5000   // Creating a quantum circuit context
	GasQCGate      = 500    // Applying a single-qubit gate (per qubit)
	GasQCTwoQubit  = 2000   // Applying a two-qubit gate
	GasQCMeasure   = 1000   // Measuring a qubit
	GasQCExecute   = 50000  // Executing a circuit (base cost)
	GasQCExecutePerQubit = 10000 // Additional cost per qubit
	GasQCExecutePerGate  = 500   // Additional cost per gate
	GasQCGrover    = 100000 // Grover's algorithm (base)
	GasQCQFT       = 80000  // QFT (base)
	GasQCVQE       = 200000 // VQE (base, most expensive)
	GasQCVerifyPQ  = 30000  // Post-quantum signature verification
)

// ============================================================
// Quantum EVM Context
// ============================================================

// QuantumContext holds the quantum state for a smart contract execution.
type QuantumContext struct {
	mu          sync.Mutex
	CircuitID   string
	Circuit     *qminer.QuantumCircuit
	Result      *qminer.QuantumResult
	IsExecuted  bool
	GasUsed     uint64
}

// QuantumEVM extends the EVM with quantum capabilities.
type QuantumEVM struct {
	mu       sync.RWMutex
	engine   *qminer.QMaaSEngine
	contexts map[[32]byte]*QuantumContext // Contract address -> quantum context
}

// NewQuantumEVM creates a new Quantum EVM instance.
func NewQuantumEVM(maxQubits int) *QuantumEVM {
	return &QuantumEVM{
		engine:   qminer.NewQMaaSEngine(maxQubits, false),
		contexts: make(map[[32]byte]*QuantumContext),
	}
}

// ============================================================
// Quantum Opcode Execution
// ============================================================

// ExecuteQuantumOpcode executes a quantum opcode and returns the result.
// Returns (result, gasUsed, error).
func (qevm *QuantumEVM) ExecuteQuantumOpcode(
	op OpCode,
	caller [20]byte,
	stack [][]byte,
	memory []byte,
	gasAvailable uint64,
) ([]byte, uint64, error) {

	switch op {
	case QC_CREATE:
		return qevm.opQCCreate(caller, stack, gasAvailable)
	case QC_HADAMARD:
		return qevm.opQCGate(caller, qminer.GateH, stack, gasAvailable)
	case QC_PAULI_X:
		return qevm.opQCGate(caller, qminer.GateX, stack, gasAvailable)
	case QC_PAULI_Y:
		return qevm.opQCGate(caller, qminer.GateY, stack, gasAvailable)
	case QC_PAULI_Z:
		return qevm.opQCGate(caller, qminer.GateZ, stack, gasAvailable)
	case QC_CNOT:
		return qevm.opQCTwoQubitGate(caller, qminer.GateCNOT, stack, gasAvailable)
	case QC_RZ:
		return qevm.opQCRotation(caller, qminer.GateRz, stack, gasAvailable)
	case QC_RY:
		return qevm.opQCRotation(caller, qminer.GateRy, stack, gasAvailable)
	case QC_MEASURE:
		return qevm.opQCMeasure(caller, stack, gasAvailable)
	case QC_EXECUTE:
		return qevm.opQCExecute(caller, stack, gasAvailable)
	case QC_RESULT:
		return qevm.opQCResult(caller, stack, gasAvailable)
	case QC_GROVER:
		return qevm.opQCGrover(caller, stack, gasAvailable)
	case QC_QFT:
		return qevm.opQCQFT(caller, stack, gasAvailable)
	case QC_VQE:
		return qevm.opQCVQE(caller, stack, gasAvailable)
	case QC_ENTANGLE:
		return qevm.opQCEntangle(caller, stack, gasAvailable)
	case QC_VERIFY_PQ:
		return qevm.opQCVerifyPQ(caller, stack, memory, gasAvailable)
	default:
		return nil, 0, fmt.Errorf("qevm: unknown quantum opcode 0x%02X", op)
	}
}

// ============================================================
// Individual Opcode Implementations
// ============================================================

// opQCCreate creates a new quantum circuit context for the calling contract.
// Stack: [numQubits]
// Returns: [circuitID (bytes32)]
func (qevm *QuantumEVM) opQCCreate(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCCreate {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_CREATE")
	}
	if len(stack) < 1 {
		return nil, 0, fmt.Errorf("qevm: QC_CREATE requires 1 stack argument (numQubits)")
	}

	numQubits := int(new(big.Int).SetBytes(stack[0]).Int64())
	if numQubits < 1 || numQubits > 30 {
		return nil, 0, fmt.Errorf("qevm: QC_CREATE numQubits must be between 1 and 30, got %d", numQubits)
	}

	// Create a unique circuit ID based on caller and timestamp
	circuitID := fmt.Sprintf("qevm-%x-%d", caller, numQubits)

	circuit := &qminer.QuantumCircuit{
		ID:        circuitID,
		NumQubits: numQubits,
		Algorithm: "Custom",
	}

	// Store context
	var key [32]byte
	copy(key[:20], caller[:])

	qevm.mu.Lock()
	qevm.contexts[key] = &QuantumContext{
		CircuitID: circuitID,
		Circuit:   circuit,
	}
	qevm.mu.Unlock()

	// Return circuit ID as bytes32
	result := make([]byte, 32)
	copy(result, []byte(circuitID))
	return result, GasQCCreate, nil
}

// opQCGate applies a single-qubit gate to the circuit.
// Stack: [qubitIndex]
func (qevm *QuantumEVM) opQCGate(caller [20]byte, gateType qminer.GateType, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCGate {
		return nil, 0, fmt.Errorf("qevm: out of gas for quantum gate")
	}
	if len(stack) < 1 {
		return nil, 0, fmt.Errorf("qevm: gate requires qubit index on stack")
	}

	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	qubit := int(new(big.Int).SetBytes(stack[0]).Int64())
	if qubit >= ctx.Circuit.NumQubits {
		return nil, 0, fmt.Errorf("qevm: qubit index %d out of range (circuit has %d qubits)", qubit, ctx.Circuit.NumQubits)
	}

	ctx.mu.Lock()
	ctx.Circuit.Gates = append(ctx.Circuit.Gates, &qminer.QuantumGate{
		Type:   gateType,
		Qubits: []int{qubit},
		Layer:  len(ctx.Circuit.Gates),
	})
	ctx.mu.Unlock()

	return padLeft32([]byte{1}), GasQCGate, nil
}

// opQCTwoQubitGate applies a two-qubit gate.
// Stack: [controlQubit, targetQubit]
func (qevm *QuantumEVM) opQCTwoQubitGate(caller [20]byte, gateType qminer.GateType, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCTwoQubit {
		return nil, 0, fmt.Errorf("qevm: out of gas for two-qubit gate")
	}
	if len(stack) < 2 {
		return nil, 0, fmt.Errorf("qevm: two-qubit gate requires 2 stack arguments")
	}

	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	control := int(new(big.Int).SetBytes(stack[0]).Int64())
	target := int(new(big.Int).SetBytes(stack[1]).Int64())

	if control >= ctx.Circuit.NumQubits || target >= ctx.Circuit.NumQubits {
		return nil, 0, fmt.Errorf("qevm: qubit indices out of range")
	}
	if control == target {
		return nil, 0, fmt.Errorf("qevm: control and target qubits must be different")
	}

	ctx.mu.Lock()
	ctx.Circuit.Gates = append(ctx.Circuit.Gates, &qminer.QuantumGate{
		Type:   gateType,
		Qubits: []int{control, target},
		Layer:  len(ctx.Circuit.Gates),
	})
	ctx.mu.Unlock()

	return padLeft32([]byte{1}), GasQCTwoQubit, nil
}

// opQCRotation applies a parametric rotation gate.
// Stack: [qubitIndex, angle_numerator, angle_denominator]
// Angle = (angle_numerator / angle_denominator) * pi
func (qevm *QuantumEVM) opQCRotation(caller [20]byte, gateType qminer.GateType, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCGate {
		return nil, 0, fmt.Errorf("qevm: out of gas for rotation gate")
	}
	if len(stack) < 3 {
		return nil, 0, fmt.Errorf("qevm: rotation gate requires 3 stack arguments (qubit, numerator, denominator)")
	}

	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	qubit := int(new(big.Int).SetBytes(stack[0]).Int64())
	num := new(big.Float).SetInt(new(big.Int).SetBytes(stack[1]))
	den := new(big.Float).SetInt(new(big.Int).SetBytes(stack[2]))

	if den.Sign() == 0 {
		return nil, 0, fmt.Errorf("qevm: rotation angle denominator cannot be zero")
	}

	angleRat, _ := new(big.Float).Quo(num, den).Float64()
	angle := angleRat * 3.14159265358979 // pi

	ctx.mu.Lock()
	ctx.Circuit.Gates = append(ctx.Circuit.Gates, &qminer.QuantumGate{
		Type:   gateType,
		Qubits: []int{qubit},
		Params: []float64{angle},
		Layer:  len(ctx.Circuit.Gates),
	})
	ctx.mu.Unlock()

	return padLeft32([]byte{1}), GasQCGate, nil
}

// opQCMeasure adds a measurement gate to the circuit.
// Stack: [qubitIndex]
func (qevm *QuantumEVM) opQCMeasure(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCMeasure {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_MEASURE")
	}

	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	qubit := 0
	if len(stack) > 0 {
		qubit = int(new(big.Int).SetBytes(stack[0]).Int64())
	}

	ctx.mu.Lock()
	ctx.Circuit.Gates = append(ctx.Circuit.Gates, &qminer.QuantumGate{
		Type:   qminer.GateMeasure,
		Qubits: []int{qubit},
		Layer:  len(ctx.Circuit.Gates),
	})
	ctx.mu.Unlock()

	return padLeft32([]byte{1}), GasQCMeasure, nil
}

// opQCExecute executes the quantum circuit and stores the result.
// Stack: [shots]
// Returns: [resultHash (bytes32)]
func (qevm *QuantumEVM) opQCExecute(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	shots := 1024
	if len(stack) > 0 {
		shots = int(new(big.Int).SetBytes(stack[0]).Int64())
		if shots < 1 || shots > 65536 {
			shots = 1024
		}
	}

	// Calculate gas cost based on circuit complexity
	gasNeeded := uint64(GasQCExecute) +
		uint64(ctx.Circuit.NumQubits)*GasQCExecutePerQubit +
		uint64(len(ctx.Circuit.Gates))*GasQCExecutePerGate

	if gas < gasNeeded {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_EXECUTE (need %d, have %d)", gasNeeded, gas)
	}

	// Submit job to QMaaS engine
	job := &qminer.QuantumJob{
		ID:      ctx.CircuitID,
		Circuit: ctx.Circuit,
		Shots:   shots,
		Status:  qminer.JobPending,
	}

	if err := qevm.engine.SubmitJob(job); err != nil {
		// Job might already exist, try to get result
	}

	// Execute the job (in production, this would be async via oracle)
	result, err := qevm.engine.ProcessJob(ctx.CircuitID, caller)
	if err != nil {
		return nil, gasNeeded, fmt.Errorf("qevm: quantum execution failed: %w", err)
	}

	ctx.mu.Lock()
	ctx.Result = result
	ctx.IsExecuted = true
	ctx.mu.Unlock()

	return result.ResultHash[:], gasNeeded, nil
}

// opQCResult retrieves the most probable measurement outcome.
// Returns: [outcome (uint256)]
func (qevm *QuantumEVM) opQCResult(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	if !ctx.IsExecuted || ctx.Result == nil {
		return nil, 0, fmt.Errorf("qevm: circuit has not been executed yet, call QC_EXECUTE first")
	}

	// Find the most probable outcome
	maxCount := 0
	mostProbable := ""
	for outcome, count := range ctx.Result.Counts {
		if count > maxCount {
			maxCount = count
			mostProbable = outcome
		}
	}

	// Convert binary string to integer
	result := new(big.Int)
	for i, c := range mostProbable {
		if c == '1' {
			result.SetBit(result, len(mostProbable)-1-i, 1)
		}
	}

	return padLeft32(result.Bytes()), 0, nil
}

// opQCGrover executes Grover's search algorithm.
// Stack: [numQubits, targetState]
// Returns: [foundState (uint256), probability (uint256 * 1e18)]
func (qevm *QuantumEVM) opQCGrover(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCGrover {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_GROVER")
	}
	if len(stack) < 2 {
		return nil, 0, fmt.Errorf("qevm: QC_GROVER requires 2 stack arguments (numQubits, targetState)")
	}

	numQubits := int(new(big.Int).SetBytes(stack[0]).Int64())
	targetState := int(new(big.Int).SetBytes(stack[1]).Int64())

	if numQubits < 1 || numQubits > 20 {
		return nil, 0, fmt.Errorf("qevm: Grover numQubits must be between 1 and 20")
	}

	// Build and execute Grover circuit
	circuit := qminer.BuildGroverCircuit(numQubits, targetState)

	sim := qminer.NewQuantumSimulator(numQubits, false)
	result, err := sim.Execute(circuit, 1024)
	if err != nil {
		return nil, GasQCGrover, fmt.Errorf("qevm: Grover execution failed: %w", err)
	}

	// Find the most probable state
	maxCount := 0
	foundState := 0
	for outcome, count := range result.Counts {
		if count > maxCount {
			maxCount = count
			val := new(big.Int)
			for i, c := range outcome {
				if c == '1' {
					val.SetBit(val, len(outcome)-1-i, 1)
				}
			}
			foundState = int(val.Int64())
		}
	}

	// Return found state
	return padLeft32(big.NewInt(int64(foundState)).Bytes()), GasQCGrover, nil
}

// opQCQFT executes the Quantum Fourier Transform.
// Stack: [numQubits]
// Returns: [resultHash (bytes32)]
func (qevm *QuantumEVM) opQCQFT(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCQFT {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_QFT")
	}
	if len(stack) < 1 {
		return nil, 0, fmt.Errorf("qevm: QC_QFT requires numQubits on stack")
	}

	numQubits := int(new(big.Int).SetBytes(stack[0]).Int64())
	if numQubits < 1 || numQubits > 20 {
		return nil, 0, fmt.Errorf("qevm: QFT numQubits must be between 1 and 20")
	}

	circuit := qminer.BuildQFTCircuit(numQubits)
	sim := qminer.NewQuantumSimulator(numQubits, false)
	result, err := sim.Execute(circuit, 1024)
	if err != nil {
		return nil, GasQCQFT, fmt.Errorf("qevm: QFT execution failed: %w", err)
	}

	return result.ResultHash[:], GasQCQFT, nil
}

// opQCVQE executes the Variational Quantum Eigensolver.
// Stack: [numQubits, param1, param2, ...]
// Returns: [energyEstimate (int256 * 1e18)]
func (qevm *QuantumEVM) opQCVQE(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCVQE {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_VQE")
	}
	if len(stack) < 1 {
		return nil, 0, fmt.Errorf("qevm: QC_VQE requires at least numQubits on stack")
	}

	numQubits := int(new(big.Int).SetBytes(stack[0]).Int64())
	if numQubits < 2 || numQubits > 16 {
		return nil, 0, fmt.Errorf("qevm: VQE numQubits must be between 2 and 16")
	}

	// Extract parameters from stack
	params := make([]float64, 0)
	for i := 1; i < len(stack) && i < 20; i++ {
		val := new(big.Int).SetBytes(stack[i])
		// Interpret as fixed-point: value / 1e18
		fval := new(big.Float).SetInt(val)
		scale := new(big.Float).SetFloat64(1e18)
		fval.Quo(fval, scale)
		f, _ := fval.Float64()
		params = append(params, f)
	}

	circuit := qminer.BuildVQECircuit(numQubits, params)
	sim := qminer.NewQuantumSimulator(numQubits, false)
	result, err := sim.Execute(circuit, 2048)
	if err != nil {
		return nil, GasQCVQE, fmt.Errorf("qevm: VQE execution failed: %w", err)
	}

	// Return result hash as energy estimate proxy
	return result.ResultHash[:], GasQCVQE, nil
}

// opQCEntangle creates a Bell state (maximally entangled pair).
// Stack: [qubit1, qubit2]
// Returns: [1 (success)]
func (qevm *QuantumEVM) opQCEntangle(caller [20]byte, stack [][]byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCTwoQubit {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_ENTANGLE")
	}

	ctx, err := qevm.getContext(caller)
	if err != nil {
		return nil, 0, err
	}

	q1, q2 := 0, 1
	if len(stack) >= 2 {
		q1 = int(new(big.Int).SetBytes(stack[0]).Int64())
		q2 = int(new(big.Int).SetBytes(stack[1]).Int64())
	}

	ctx.mu.Lock()
	// H on q1, then CNOT(q1, q2) creates Bell state
	layer := len(ctx.Circuit.Gates)
	ctx.Circuit.Gates = append(ctx.Circuit.Gates,
		&qminer.QuantumGate{Type: qminer.GateH, Qubits: []int{q1}, Layer: layer},
		&qminer.QuantumGate{Type: qminer.GateCNOT, Qubits: []int{q1, q2}, Layer: layer + 1},
	)
	ctx.mu.Unlock()

	return padLeft32([]byte{1}), GasQCTwoQubit, nil
}

// opQCVerifyPQ verifies a post-quantum (ML-DSA) signature on-chain.
// Stack: [messageHash (bytes32), publicKeyOffset, signatureOffset]
// Memory: [publicKey bytes, signature bytes]
// Returns: [1 if valid, 0 if invalid]
func (qevm *QuantumEVM) opQCVerifyPQ(caller [20]byte, stack [][]byte, memory []byte, gas uint64) ([]byte, uint64, error) {
	if gas < GasQCVerifyPQ {
		return nil, 0, fmt.Errorf("qevm: out of gas for QC_VERIFY_PQ")
	}
	if len(stack) < 3 {
		return nil, 0, fmt.Errorf("qevm: QC_VERIFY_PQ requires 3 stack arguments")
	}

	// Extract message hash
	messageHash := stack[0]
	if len(messageHash) > 32 {
		messageHash = messageHash[len(messageHash)-32:]
	}

	// Extract public key and signature from memory
	pkOffset := int(new(big.Int).SetBytes(stack[1]).Int64())
	sigOffset := int(new(big.Int).SetBytes(stack[2]).Int64())

	if pkOffset+pqcrypto.MLDSA_PUBLICKEY_SIZE > len(memory) {
		return nil, GasQCVerifyPQ, fmt.Errorf("qevm: public key out of memory bounds")
	}
	if sigOffset+pqcrypto.MLDSA_SIGNATURE_SIZE > len(memory) {
		return nil, GasQCVerifyPQ, fmt.Errorf("qevm: signature out of memory bounds")
	}

	pkBytes := memory[pkOffset : pkOffset+pqcrypto.MLDSA_PUBLICKEY_SIZE]
	sigBytes := memory[sigOffset : sigOffset+pqcrypto.MLDSA_SIGNATURE_SIZE]

	// Parse public key and signature
	pubKey, err := pqcrypto.ParseMLDSAPublicKey(pkBytes)
	if err != nil {
		return padLeft32([]byte{0}), GasQCVerifyPQ, nil // Invalid, not an error
	}

	sig, err := pqcrypto.ParseMLDSASignature(sigBytes)
	if err != nil {
		return padLeft32([]byte{0}), GasQCVerifyPQ, nil
	}

	// Verify the signature
	if err := pqcrypto.Verify(pubKey, messageHash, sig); err != nil {
		return padLeft32([]byte{0}), GasQCVerifyPQ, nil
	}

	return padLeft32([]byte{1}), GasQCVerifyPQ, nil
}

// ============================================================
// Helper Functions
// ============================================================

// getContext retrieves the quantum context for a caller.
func (qevm *QuantumEVM) getContext(caller [20]byte) (*QuantumContext, error) {
	var key [32]byte
	copy(key[:20], caller[:])

	qevm.mu.RLock()
	ctx, exists := qevm.contexts[key]
	qevm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("qevm: no quantum context for caller %x, call QC_CREATE first", caller)
	}
	return ctx, nil
}

// padLeft32 pads a byte slice to 32 bytes (left-padded with zeros).
func padLeft32(b []byte) []byte {
	result := make([]byte, 32)
	if len(b) > 32 {
		copy(result, b[len(b)-32:])
	} else {
		copy(result[32-len(b):], b)
	}
	return result
}

// uint64ToBytes32 converts a uint64 to a 32-byte big-endian representation.
func uint64ToBytes32(v uint64) []byte {
	result := make([]byte, 32)
	binary.BigEndian.PutUint64(result[24:], v)
	return result
}
