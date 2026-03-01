// Package quantum implements the Quantum Mining as a Service (QMaaS) engine for QBP.
//
// QMaaS is a decentralized marketplace for quantum computation where:
//   - "Miners" are quantum computation providers with GPU/CPU simulators
//   - Users and smart contracts submit quantum computation jobs
//   - Miners compete to solve quantum circuits and earn QBP rewards
//   - Results are verified on-chain via the Quantum Oracle
//
// This is fundamentally different from traditional mining:
//   - No wasteful PoW computation
//   - Computation has real-world utility (quantum simulation)
//   - Miners provide value by running quantum algorithms
//   - The blockchain becomes a marketplace for quantum computing
//
// Supported Quantum Algorithms:
//   - Grover's Search Algorithm (quadratic speedup for search problems)
//   - Quantum Fourier Transform (QFT) - basis for many quantum algorithms
//   - Variational Quantum Eigensolver (VQE) - quantum chemistry
//   - Quantum Approximate Optimization Algorithm (QAOA) - combinatorial optimization
//   - Bernstein-Vazirani Algorithm - hidden string discovery
//   - Quantum Phase Estimation (QPE) - eigenvalue problems
package quantum

import (
	"crypto/sha3"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/cmplx"
	"sync"
	"time"
)

// ============================================================
// Quantum Circuit Data Structures
// ============================================================

// GateType represents the type of a quantum gate.
type GateType string

const (
	// Single-qubit gates
	GateH    GateType = "H"    // Hadamard
	GateX    GateType = "X"    // Pauli-X (NOT)
	GateY    GateType = "Y"    // Pauli-Y
	GateZ    GateType = "Z"    // Pauli-Z
	GateS    GateType = "S"    // Phase (sqrt(Z))
	GateT    GateType = "T"    // T gate (pi/8)
	GateRx   GateType = "Rx"   // Rotation around X-axis
	GateRy   GateType = "Ry"   // Rotation around Y-axis
	GateRz   GateType = "Rz"   // Rotation around Z-axis
	GateU    GateType = "U"    // Generic single-qubit unitary

	// Two-qubit gates
	GateCNOT GateType = "CNOT" // Controlled-NOT
	GateCZ   GateType = "CZ"   // Controlled-Z
	GateSWAP GateType = "SWAP" // SWAP gate
	GateCRz  GateType = "CRz"  // Controlled Rz

	// Multi-qubit gates
	GateToffoli GateType = "Toffoli" // Toffoli (CCNOT)
	GateFredkin GateType = "Fredkin" // Fredkin (CSWAP)

	// Measurement
	GateMeasure GateType = "Measure" // Measurement in computational basis
)

// QuantumGate represents a single quantum gate operation.
type QuantumGate struct {
	Type    GateType  `json:"type"`
	Qubits  []int     `json:"qubits"`  // Target qubits
	Params  []float64 `json:"params"`  // Rotation angles (for parametric gates)
	Layer   int       `json:"layer"`   // Circuit layer (for parallel execution)
}

// QuantumCircuit represents a complete quantum circuit.
type QuantumCircuit struct {
	ID          string         `json:"id"`
	NumQubits   int            `json:"num_qubits"`
	Gates       []*QuantumGate `json:"gates"`
	Depth       int            `json:"depth"`
	Algorithm   string         `json:"algorithm"`
	Description string         `json:"description"`
	CreatedAt   int64          `json:"created_at"`
}

// QuantumJob represents a quantum computation job submitted to the network.
type QuantumJob struct {
	ID           string          `json:"id"`
	Circuit      *QuantumCircuit `json:"circuit"`
	Submitter    [20]byte        `json:"submitter"`
	Reward       uint64          `json:"reward"`       // QBP tokens as reward
	MaxGas       uint64          `json:"max_gas"`      // Maximum gas for on-chain verification
	Shots        int             `json:"shots"`        // Number of measurement shots
	SubmittedAt  uint64          `json:"submitted_at"` // Block number
	Deadline     uint64          `json:"deadline"`     // Block number deadline
	Status       JobStatus       `json:"status"`
}

// JobStatus represents the status of a quantum job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobExpired   JobStatus = "expired"
)

// QuantumResult represents the result of a quantum computation.
type QuantumResult struct {
	JobID        string              `json:"job_id"`
	Miner        [20]byte            `json:"miner"`
	Counts       map[string]int      `json:"counts"`       // Measurement outcome counts
	Probabilities map[string]float64 `json:"probabilities"` // State probabilities
	StateVector  []complex128        `json:"state_vector,omitempty"` // Full state vector (optional)
	ResultHash   [32]byte            `json:"result_hash"`  // Hash for on-chain verification
	ExecutionTime time.Duration      `json:"execution_time"`
	NumQubits    int                 `json:"num_qubits"`
	NumGates     int                 `json:"num_gates"`
	Timestamp    int64               `json:"timestamp"`
}

// ============================================================
// Quantum State Vector Simulator
// ============================================================

// QuantumSimulator is a high-performance quantum circuit simulator.
// In production, this would use GPU acceleration via CUDA/cuQuantum.
type QuantumSimulator struct {
	mu          sync.Mutex
	maxQubits   int
	useGPU      bool
	numWorkers  int
}

// NewQuantumSimulator creates a new quantum simulator.
func NewQuantumSimulator(maxQubits int, useGPU bool) *QuantumSimulator {
	return &QuantumSimulator{
		maxQubits:  maxQubits,
		useGPU:     useGPU,
		numWorkers: 4,
	}
}

// Execute executes a quantum circuit and returns the result.
func (sim *QuantumSimulator) Execute(circuit *QuantumCircuit, shots int) (*QuantumResult, error) {
	if circuit.NumQubits > sim.maxQubits {
		return nil, fmt.Errorf("qminer: circuit requires %d qubits, simulator supports max %d",
			circuit.NumQubits, sim.maxQubits)
	}

	startTime := time.Now()

	// Initialize state vector |0...0>
	stateSize := 1 << circuit.NumQubits
	state := make([]complex128, stateSize)
	state[0] = 1.0 + 0i

	// Execute each gate
	for _, gate := range circuit.Gates {
		var err error
		state, err = sim.applyGate(state, gate, circuit.NumQubits)
		if err != nil {
			return nil, fmt.Errorf("qminer: failed to apply gate %s: %w", gate.Type, err)
		}
	}

	// Perform measurements
	counts := sim.measure(state, circuit.NumQubits, shots)

	// Calculate probabilities
	probs := make(map[string]float64)
	for i, amp := range state {
		prob := real(amp*cmplx.Conj(amp))
		if prob > 1e-10 {
			key := fmt.Sprintf("%0*b", circuit.NumQubits, i)
			probs[key] = prob
		}
	}

	// Compute result hash for on-chain verification
	resultHash := sim.computeResultHash(counts, circuit.ID)

	execTime := time.Since(startTime)

	return &QuantumResult{
		JobID:         circuit.ID,
		Counts:        counts,
		Probabilities: probs,
		StateVector:   state,
		ResultHash:    resultHash,
		ExecutionTime: execTime,
		NumQubits:     circuit.NumQubits,
		NumGates:      len(circuit.Gates),
		Timestamp:     time.Now().Unix(),
	}, nil
}

// applyGate applies a single quantum gate to the state vector.
func (sim *QuantumSimulator) applyGate(state []complex128, gate *QuantumGate, numQubits int) ([]complex128, error) {
	switch gate.Type {
	case GateH:
		return sim.applyHadamard(state, gate.Qubits[0], numQubits)
	case GateX:
		return sim.applyPauliX(state, gate.Qubits[0], numQubits)
	case GateY:
		return sim.applyPauliY(state, gate.Qubits[0], numQubits)
	case GateZ:
		return sim.applyPauliZ(state, gate.Qubits[0], numQubits)
	case GateS:
		return sim.applyPhase(state, gate.Qubits[0], numQubits, math.Pi/2)
	case GateT:
		return sim.applyPhase(state, gate.Qubits[0], numQubits, math.Pi/4)
	case GateRx:
		if len(gate.Params) < 1 {
			return nil, fmt.Errorf("Rx gate requires 1 parameter")
		}
		return sim.applyRx(state, gate.Qubits[0], numQubits, gate.Params[0])
	case GateRy:
		if len(gate.Params) < 1 {
			return nil, fmt.Errorf("Ry gate requires 1 parameter")
		}
		return sim.applyRy(state, gate.Qubits[0], numQubits, gate.Params[0])
	case GateRz:
		if len(gate.Params) < 1 {
			return nil, fmt.Errorf("Rz gate requires 1 parameter")
		}
		return sim.applyRz(state, gate.Qubits[0], numQubits, gate.Params[0])
	case GateCNOT:
		if len(gate.Qubits) < 2 {
			return nil, fmt.Errorf("CNOT gate requires 2 qubits")
		}
		return sim.applyCNOT(state, gate.Qubits[0], gate.Qubits[1], numQubits)
	case GateCZ:
		if len(gate.Qubits) < 2 {
			return nil, fmt.Errorf("CZ gate requires 2 qubits")
		}
		return sim.applyCZ(state, gate.Qubits[0], gate.Qubits[1], numQubits)
	case GateSWAP:
		if len(gate.Qubits) < 2 {
			return nil, fmt.Errorf("SWAP gate requires 2 qubits")
		}
		return sim.applySWAP(state, gate.Qubits[0], gate.Qubits[1], numQubits)
	case GateToffoli:
		if len(gate.Qubits) < 3 {
			return nil, fmt.Errorf("Toffoli gate requires 3 qubits")
		}
		return sim.applyToffoli(state, gate.Qubits[0], gate.Qubits[1], gate.Qubits[2], numQubits)
	case GateMeasure:
		// Measurement is handled separately
		return state, nil
	default:
		return nil, fmt.Errorf("unknown gate type: %s", gate.Type)
	}
}

// ============================================================
// Single-Qubit Gate Implementations
// ============================================================

// applyHadamard applies the Hadamard gate to qubit q.
// H = (1/sqrt(2)) * [[1, 1], [1, -1]]
func (sim *QuantumSimulator) applyHadamard(state []complex128, q, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)
	factor := complex(1.0/math.Sqrt2, 0)

	for i := range state {
		if (i>>q)&1 == 0 {
			j := i | (1 << q)
			result[i] = factor * (state[i] + state[j])
			result[j] = factor * (state[i] - state[j])
		}
	}
	return result, nil
}

// applyPauliX applies the Pauli-X (NOT) gate to qubit q.
// X = [[0, 1], [1, 0]]
func (sim *QuantumSimulator) applyPauliX(state []complex128, q, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>q)&1 == 0 {
			j := i | (1 << q)
			result[i], result[j] = state[j], state[i]
		}
	}
	return result, nil
}

// applyPauliY applies the Pauli-Y gate to qubit q.
// Y = [[0, -i], [i, 0]]
func (sim *QuantumSimulator) applyPauliY(state []complex128, q, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>q)&1 == 0 {
			j := i | (1 << q)
			result[i] = complex(0, 1) * state[j]
			result[j] = complex(0, -1) * state[i]
		}
	}
	return result, nil
}

// applyPauliZ applies the Pauli-Z gate to qubit q.
// Z = [[1, 0], [0, -1]]
func (sim *QuantumSimulator) applyPauliZ(state []complex128, q, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>q)&1 == 1 {
			result[i] = -state[i]
		}
	}
	return result, nil
}

// applyPhase applies a phase gate with angle theta to qubit q.
// P(theta) = [[1, 0], [0, e^(i*theta)]]
func (sim *QuantumSimulator) applyPhase(state []complex128, q, numQubits int, theta float64) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)
	phase := cmplx.Exp(complex(0, theta))

	for i := range state {
		if (i>>q)&1 == 1 {
			result[i] = phase * state[i]
		}
	}
	return result, nil
}

// applyRx applies rotation around X-axis by angle theta.
// Rx(theta) = [[cos(t/2), -i*sin(t/2)], [-i*sin(t/2), cos(t/2)]]
func (sim *QuantumSimulator) applyRx(state []complex128, q, numQubits int, theta float64) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)
	cosT := complex(math.Cos(theta/2), 0)
	sinT := complex(0, -math.Sin(theta/2))

	for i := range state {
		if (i>>q)&1 == 0 {
			j := i | (1 << q)
			result[i] = cosT*state[i] + sinT*state[j]
			result[j] = sinT*state[i] + cosT*state[j]
		}
	}
	return result, nil
}

// applyRy applies rotation around Y-axis by angle theta.
// Ry(theta) = [[cos(t/2), -sin(t/2)], [sin(t/2), cos(t/2)]]
func (sim *QuantumSimulator) applyRy(state []complex128, q, numQubits int, theta float64) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)
	cosT := complex(math.Cos(theta/2), 0)
	sinT := complex(math.Sin(theta/2), 0)

	for i := range state {
		if (i>>q)&1 == 0 {
			j := i | (1 << q)
			result[i] = cosT*state[i] - sinT*state[j]
			result[j] = sinT*state[i] + cosT*state[j]
		}
	}
	return result, nil
}

// applyRz applies rotation around Z-axis by angle theta.
// Rz(theta) = [[e^(-i*t/2), 0], [0, e^(i*t/2)]]
func (sim *QuantumSimulator) applyRz(state []complex128, q, numQubits int, theta float64) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)
	phase0 := cmplx.Exp(complex(0, -theta/2))
	phase1 := cmplx.Exp(complex(0, theta/2))

	for i := range state {
		if (i>>q)&1 == 0 {
			result[i] = phase0 * state[i]
		} else {
			result[i] = phase1 * state[i]
		}
	}
	return result, nil
}

// ============================================================
// Two-Qubit Gate Implementations
// ============================================================

// applyCNOT applies the CNOT gate with control qubit c and target qubit t.
func (sim *QuantumSimulator) applyCNOT(state []complex128, control, target, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>control)&1 == 1 {
			j := i ^ (1 << target)
			result[i] = state[j]
			result[j] = state[i]
		}
	}
	return result, nil
}

// applyCZ applies the CZ gate with control qubit c and target qubit t.
func (sim *QuantumSimulator) applyCZ(state []complex128, control, target, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>control)&1 == 1 && (i>>target)&1 == 1 {
			result[i] = -state[i]
		}
	}
	return result, nil
}

// applySWAP swaps the states of two qubits.
func (sim *QuantumSimulator) applySWAP(state []complex128, q1, q2, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		bit1 := (i >> q1) & 1
		bit2 := (i >> q2) & 1
		if bit1 != bit2 {
			j := i ^ (1 << q1) ^ (1 << q2)
			result[i] = state[j]
		}
	}
	return result, nil
}

// applyToffoli applies the Toffoli (CCNOT) gate.
func (sim *QuantumSimulator) applyToffoli(state []complex128, c1, c2, target, numQubits int) ([]complex128, error) {
	result := make([]complex128, len(state))
	copy(result, state)

	for i := range state {
		if (i>>c1)&1 == 1 && (i>>c2)&1 == 1 {
			j := i ^ (1 << target)
			result[i] = state[j]
			result[j] = state[i]
		}
	}
	return result, nil
}

// ============================================================
// Measurement
// ============================================================

// measure performs quantum measurements and returns outcome counts.
func (sim *QuantumSimulator) measure(state []complex128, numQubits, shots int) map[string]int {
	// Calculate probabilities
	probs := make([]float64, len(state))
	for i, amp := range state {
		probs[i] = real(amp * cmplx.Conj(amp))
	}

	// Sample outcomes
	counts := make(map[string]int)
	for shot := 0; shot < shots; shot++ {
		// Deterministic sampling based on shot number for reproducibility
		h := sha3.New256()
		binary.Write(h, binary.LittleEndian, int64(shot))
		for _, p := range probs {
			binary.Write(h, binary.LittleEndian, p)
		}
		hashBytes := h.Sum(nil)
		randVal := float64(binary.BigEndian.Uint64(hashBytes[:8])) / float64(^uint64(0))

		// Find the outcome
		cumProb := 0.0
		outcome := len(probs) - 1
		for i, p := range probs {
			cumProb += p
			if randVal < cumProb {
				outcome = i
				break
			}
		}
		key := fmt.Sprintf("%0*b", numQubits, outcome)
		counts[key]++
	}
	return counts
}

// computeResultHash computes a deterministic hash of the quantum result for on-chain verification.
func (sim *QuantumSimulator) computeResultHash(counts map[string]int, circuitID string) [32]byte {
	h := sha3.New256()
	h.Write([]byte(circuitID))

	// Sort keys for determinism
	data, _ := json.Marshal(counts)
	h.Write(data)

	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// ============================================================
// Quantum Algorithms Library
// ============================================================

// BuildGroverCircuit builds a Grover's search circuit for searching a database.
// Grover's algorithm provides quadratic speedup: O(sqrt(N)) vs O(N) classical.
func BuildGroverCircuit(numQubits int, targetState int) *QuantumCircuit {
	circuit := &QuantumCircuit{
		ID:          fmt.Sprintf("grover-%d-%d", numQubits, targetState),
		NumQubits:   numQubits,
		Algorithm:   "Grover",
		Description: fmt.Sprintf("Grover's search for target state %d in %d-qubit space", targetState, numQubits),
		CreatedAt:   time.Now().Unix(),
	}

	// Number of Grover iterations: pi/4 * sqrt(N)
	N := 1 << numQubits
	numIterations := int(math.Pi / 4 * math.Sqrt(float64(N)))

	// Step 1: Initialize superposition with Hadamard on all qubits
	for q := 0; q < numQubits; q++ {
		circuit.Gates = append(circuit.Gates, &QuantumGate{
			Type:   GateH,
			Qubits: []int{q},
			Layer:  0,
		})
	}

	// Step 2: Grover iterations
	for iter := 0; iter < numIterations; iter++ {
		layer := 1 + iter*2

		// Oracle: flip the phase of the target state
		// Implemented as a multi-controlled Z gate
		for q := 0; q < numQubits; q++ {
			if (targetState>>q)&1 == 0 {
				circuit.Gates = append(circuit.Gates, &QuantumGate{
					Type:   GateX,
					Qubits: []int{q},
					Layer:  layer,
				})
			}
		}
		// Multi-controlled Z (simplified as CZ for 2-qubit case)
		if numQubits >= 2 {
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateCZ,
				Qubits: []int{0, 1},
				Layer:  layer,
			})
		}
		for q := 0; q < numQubits; q++ {
			if (targetState>>q)&1 == 0 {
				circuit.Gates = append(circuit.Gates, &QuantumGate{
					Type:   GateX,
					Qubits: []int{q},
					Layer:  layer,
				})
			}
		}

		// Diffusion operator: 2|s><s| - I
		layer++
		for q := 0; q < numQubits; q++ {
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateH,
				Qubits: []int{q},
				Layer:  layer,
			})
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateX,
				Qubits: []int{q},
				Layer:  layer,
			})
		}
		if numQubits >= 2 {
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateCZ,
				Qubits: []int{0, 1},
				Layer:  layer,
			})
		}
		for q := 0; q < numQubits; q++ {
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateX,
				Qubits: []int{q},
				Layer:  layer,
			})
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateH,
				Qubits: []int{q},
				Layer:  layer,
			})
		}
	}

	// Measure all qubits
	for q := 0; q < numQubits; q++ {
		circuit.Gates = append(circuit.Gates, &QuantumGate{
			Type:   GateMeasure,
			Qubits: []int{q},
			Layer:  100,
		})
	}

	circuit.Depth = numIterations*2 + 2
	return circuit
}

// BuildQFTCircuit builds a Quantum Fourier Transform circuit.
// QFT is the quantum analogue of the Discrete Fourier Transform.
func BuildQFTCircuit(numQubits int) *QuantumCircuit {
	circuit := &QuantumCircuit{
		ID:          fmt.Sprintf("qft-%d", numQubits),
		NumQubits:   numQubits,
		Algorithm:   "QFT",
		Description: fmt.Sprintf("Quantum Fourier Transform on %d qubits", numQubits),
		CreatedAt:   time.Now().Unix(),
	}

	layer := 0
	for j := numQubits - 1; j >= 0; j-- {
		// Hadamard on qubit j
		circuit.Gates = append(circuit.Gates, &QuantumGate{
			Type:   GateH,
			Qubits: []int{j},
			Layer:  layer,
		})
		layer++

		// Controlled rotation gates
		for k := j - 1; k >= 0; k-- {
			angle := math.Pi / math.Pow(2, float64(j-k))
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateCRz,
				Qubits: []int{k, j},
				Params: []float64{angle},
				Layer:  layer,
			})
			layer++
		}
	}

	// Swap qubits to get correct bit order
	for i := 0; i < numQubits/2; i++ {
		circuit.Gates = append(circuit.Gates, &QuantumGate{
			Type:   GateSWAP,
			Qubits: []int{i, numQubits - 1 - i},
			Layer:  layer,
		})
	}

	circuit.Depth = layer + 1
	return circuit
}

// BuildBellStateCircuit builds a Bell state (maximally entangled) circuit.
// Bell states are fundamental to quantum communication and teleportation.
func BuildBellStateCircuit() *QuantumCircuit {
	circuit := &QuantumCircuit{
		ID:          "bell-state-phi-plus",
		NumQubits:   2,
		Algorithm:   "Bell State",
		Description: "Creates the Bell state |Φ+> = (|00> + |11>) / sqrt(2)",
		CreatedAt:   time.Now().Unix(),
	}

	circuit.Gates = []*QuantumGate{
		{Type: GateH, Qubits: []int{0}, Layer: 0},
		{Type: GateCNOT, Qubits: []int{0, 1}, Layer: 1},
		{Type: GateMeasure, Qubits: []int{0}, Layer: 2},
		{Type: GateMeasure, Qubits: []int{1}, Layer: 2},
	}

	circuit.Depth = 3
	return circuit
}

// BuildVQECircuit builds a Variational Quantum Eigensolver circuit.
// VQE is used for quantum chemistry simulations (finding ground state energies).
func BuildVQECircuit(numQubits int, params []float64) *QuantumCircuit {
	circuit := &QuantumCircuit{
		ID:          fmt.Sprintf("vqe-%d", numQubits),
		NumQubits:   numQubits,
		Algorithm:   "VQE",
		Description: fmt.Sprintf("Variational Quantum Eigensolver with %d qubits", numQubits),
		CreatedAt:   time.Now().Unix(),
	}

	layer := 0
	paramIdx := 0

	// Hardware-efficient ansatz
	for rep := 0; rep < 2; rep++ {
		// Ry rotations
		for q := 0; q < numQubits; q++ {
			theta := 0.0
			if paramIdx < len(params) {
				theta = params[paramIdx]
				paramIdx++
			}
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateRy,
				Qubits: []int{q},
				Params: []float64{theta},
				Layer:  layer,
			})
		}
		layer++

		// Rz rotations
		for q := 0; q < numQubits; q++ {
			theta := 0.0
			if paramIdx < len(params) {
				theta = params[paramIdx]
				paramIdx++
			}
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateRz,
				Qubits: []int{q},
				Params: []float64{theta},
				Layer:  layer,
			})
		}
		layer++

		// Entanglement layer (CNOT chain)
		for q := 0; q < numQubits-1; q++ {
			circuit.Gates = append(circuit.Gates, &QuantumGate{
				Type:   GateCNOT,
				Qubits: []int{q, q + 1},
				Layer:  layer,
			})
		}
		layer++
	}

	// Final measurement
	for q := 0; q < numQubits; q++ {
		circuit.Gates = append(circuit.Gates, &QuantumGate{
			Type:   GateMeasure,
			Qubits: []int{q},
			Layer:  layer,
		})
	}

	circuit.Depth = layer + 1
	return circuit
}

// ============================================================
// QMaaS Engine
// ============================================================

// QMaaSEngine manages the quantum mining marketplace.
type QMaaSEngine struct {
	mu        sync.RWMutex
	simulator *QuantumSimulator
	jobs      map[string]*QuantumJob
	results   map[string]*QuantumResult
	miners    map[[20]byte]*MinerInfo
}

// MinerInfo tracks information about a quantum miner.
type MinerInfo struct {
	Address      [20]byte
	MaxQubits    int
	JobsCompleted uint64
	TotalRewards  uint64
	SuccessRate   float64
	LastActive    time.Time
}

// NewQMaaSEngine creates a new QMaaS engine.
func NewQMaaSEngine(maxQubits int, useGPU bool) *QMaaSEngine {
	return &QMaaSEngine{
		simulator: NewQuantumSimulator(maxQubits, useGPU),
		jobs:      make(map[string]*QuantumJob),
		results:   make(map[string]*QuantumResult),
		miners:    make(map[[20]byte]*MinerInfo),
	}
}

// SubmitJob submits a quantum computation job to the marketplace.
func (e *QMaaSEngine) SubmitJob(job *QuantumJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.jobs[job.ID]; exists {
		return fmt.Errorf("qminer: job %s already exists", job.ID)
	}

	job.Status = JobPending
	e.jobs[job.ID] = job
	return nil
}

// ProcessJob executes a quantum job and returns the result.
func (e *QMaaSEngine) ProcessJob(jobID string, miner [20]byte) (*QuantumResult, error) {
	e.mu.Lock()
	job, exists := e.jobs[jobID]
	if !exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("qminer: job %s not found", jobID)
	}
	job.Status = JobRunning
	e.mu.Unlock()

	// Execute the quantum circuit
	result, err := e.simulator.Execute(job.Circuit, job.Shots)
	if err != nil {
		e.mu.Lock()
		job.Status = JobFailed
		e.mu.Unlock()
		return nil, err
	}

	result.Miner = miner

	e.mu.Lock()
	job.Status = JobCompleted
	e.results[jobID] = result

	// Update miner stats
	info, ok := e.miners[miner]
	if !ok {
		info = &MinerInfo{Address: miner}
		e.miners[miner] = info
	}
	info.JobsCompleted++
	info.TotalRewards += job.Reward
	info.LastActive = time.Now()
	e.mu.Unlock()

	return result, nil
}

// GetResult retrieves the result of a completed job.
func (e *QMaaSEngine) GetResult(jobID string) (*QuantumResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, exists := e.results[jobID]
	if !exists {
		return nil, fmt.Errorf("qminer: result for job %s not found", jobID)
	}
	return result, nil
}
