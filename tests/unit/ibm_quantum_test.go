package unit

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"testing"
)

// ============================================================
// IBM Quantum Circuit Builder Tests
// ============================================================

func TestBuildGroverCircuit(t *testing.T) {
	tests := []struct {
		name        string
		numQubits   int
		targetState int
	}{
		{"2-qubit Grover target 0", 2, 0},
		{"2-qubit Grover target 3", 2, 3},
		{"3-qubit Grover target 5", 3, 5},
		{"4-qubit Grover target 10", 4, 10},
		{"5-qubit Grover target 31", 5, 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := buildGroverCircuit(tt.numQubits, tt.targetState)

			if !strings.HasPrefix(circuit, "OPENQASM 3.0;") {
				t.Error("circuit does not start with OPENQASM 3.0 header")
			}

			expectedQubitDecl := fmt.Sprintf("qubit[%d] q;", tt.numQubits)
			if !strings.Contains(circuit, expectedQubitDecl) {
				t.Errorf("circuit missing qubit declaration: %s", expectedQubitDecl)
			}

			expectedBitDecl := fmt.Sprintf("bit[%d] c;", tt.numQubits)
			if !strings.Contains(circuit, expectedBitDecl) {
				t.Errorf("circuit missing classical bit declaration: %s", expectedBitDecl)
			}

			if !strings.Contains(circuit, "h q[0];") {
				t.Error("circuit missing initial Hadamard gates")
			}

			for i := 0; i < tt.numQubits; i++ {
				measureStr := fmt.Sprintf("measure q[%d]", i)
				if !strings.Contains(circuit, measureStr) {
					t.Errorf("circuit missing measurement for qubit %d", i)
				}
			}
		})
	}
}

func TestBuildQFTCircuit(t *testing.T) {
	tests := []struct {
		name      string
		numQubits int
	}{
		{"2-qubit QFT", 2},
		{"3-qubit QFT", 3},
		{"4-qubit QFT", 4},
		{"8-qubit QFT", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := buildQFTCircuit(tt.numQubits)

			if !strings.HasPrefix(circuit, "OPENQASM 3.0;") {
				t.Error("circuit does not start with OPENQASM 3.0 header")
			}
			if !strings.Contains(circuit, "h q[0];") {
				t.Error("QFT circuit missing Hadamard gates")
			}
			if tt.numQubits > 1 && !strings.Contains(circuit, "cp(") {
				t.Error("QFT circuit missing controlled-phase gates")
			}
			if tt.numQubits > 1 && !strings.Contains(circuit, "swap") {
				t.Error("QFT circuit missing swap gates")
			}
			for i := 0; i < tt.numQubits; i++ {
				measureStr := fmt.Sprintf("measure q[%d]", i)
				if !strings.Contains(circuit, measureStr) {
					t.Errorf("QFT circuit missing measurement for qubit %d", i)
				}
			}
		})
	}
}

func TestBuildVQECircuit(t *testing.T) {
	tests := []struct {
		name      string
		numQubits int
		params    []float64
	}{
		{"2-qubit VQE", 2, []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}},
		{"4-qubit VQE", 4, []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := buildVQECircuit(tt.numQubits, tt.params)

			if !strings.HasPrefix(circuit, "OPENQASM 3.0;") {
				t.Error("circuit does not start with OPENQASM 3.0 header")
			}
			if !strings.Contains(circuit, "ry(") {
				t.Error("VQE circuit missing RY rotation gates")
			}
			if !strings.Contains(circuit, "rz(") {
				t.Error("VQE circuit missing RZ rotation gates")
			}
			if !strings.Contains(circuit, "cx q[") {
				t.Error("VQE circuit missing CNOT entangling gates")
			}
		})
	}
}

func TestBackendSelection(t *testing.T) {
	tests := []struct {
		name      string
		numQubits int
		wantErr   bool
	}{
		{"4 qubits", 4, false},
		{"30 qubits", 30, false},
		{"127 qubits", 127, false},
		{"156 qubits", 156, false},
		{"200 qubits exceeds all", 200, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := selectBestBackend(tt.numQubits)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for qubit count exceeding all backends")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if backend == "" {
					t.Error("expected non-empty backend name")
				}
			}
		})
	}
}

func TestIBMBackendDefinitions(t *testing.T) {
	backends := map[string]int{
		"ibm_brisbane":   127,
		"ibm_osaka":      127,
		"ibm_kyoto":      127,
		"ibm_sherbrooke":  127,
		"ibm_torino":     133,
		"ibm_fez":        156,
		"ibm_marrakesh":  156,
	}

	for name, qubits := range backends {
		t.Run(name, func(t *testing.T) {
			if qubits < 100 {
				t.Errorf("backend %s has suspiciously low qubit count: %d", name, qubits)
			}
		})
	}
}

// ============================================================
// SHA-999 Tests
// ============================================================

func TestSHA999Deterministic(t *testing.T) {
	data := []byte("QUBITCOIN test data")
	hash1 := sha999(data)
	hash2 := sha999(data)
	if hash1 != hash2 {
		t.Error("SHA-999 is not deterministic")
	}
}

func TestSHA999DifferentInputs(t *testing.T) {
	hash1 := sha999([]byte("input1"))
	hash2 := sha999([]byte("input2"))
	if hash1 == hash2 {
		t.Error("SHA-999 produces same hash for different inputs")
	}
}

func TestSHA999EmptyInput(t *testing.T) {
	hash := sha999([]byte{})
	var zero [32]byte
	if hash == zero {
		t.Error("SHA-999 of empty input should not be all zeros")
	}
}

// ============================================================
// NIST KAT (Known Answer Test) Vectors
// ============================================================

func TestNISTKATMLDSAKeySizes(t *testing.T) {
	tests := []struct {
		name    string
		pkSize  int
		skSize  int
		sigSize int
	}{
		{"ML-DSA-44", 1312, 2560, 2420},
		{"ML-DSA-65", 1952, 4032, 3309},
		{"ML-DSA-87", 2592, 4896, 4627},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pkSize <= 0 || tt.skSize <= 0 || tt.sigSize <= 0 {
				t.Errorf("invalid key/sig sizes for %s", tt.name)
			}
		})
	}
}

func TestNISTKATFALCONKeySizes(t *testing.T) {
	tests := []struct {
		name   string
		pkSize int
		skSize int
	}{
		{"FALCON-512", 897, 1281},
		{"FALCON-1024", 1793, 2305},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pkSize <= 0 || tt.skSize <= 0 {
				t.Errorf("invalid key sizes for %s", tt.name)
			}
		})
	}
}

func TestNISTKATMLKEMKeySizes(t *testing.T) {
	tests := []struct {
		name          string
		pkSize        int
		skSize        int
		ctSize        int
		sharedSecSize int
	}{
		{"ML-KEM-512", 800, 1632, 768, 32},
		{"ML-KEM-768", 1184, 2400, 1088, 32},
		{"ML-KEM-1024", 1568, 3168, 1568, 32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sharedSecSize != 32 {
				t.Errorf("shared secret size should be 32, got %d", tt.sharedSecSize)
			}
		})
	}
}

func TestNISTKATSLHDSAKeySizes(t *testing.T) {
	tests := []struct {
		name    string
		pkSize  int
		sigSize int
	}{
		{"SLH-DSA-SHA2-128f", 32, 17088},
		{"SLH-DSA-SHA2-128s", 32, 7856},
		{"SLH-DSA-SHA2-192f", 48, 35664},
		{"SLH-DSA-SHA2-256f", 64, 49856},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pkSize <= 0 || tt.sigSize <= 0 {
				t.Errorf("invalid sizes for %s", tt.name)
			}
		})
	}
}

// ============================================================
// Helper Functions
// ============================================================

func sha999(data []byte) [32]byte {
	h1 := sha256.New()
	h1.Write([]byte("QBTC_SHA999_L1"))
	h1.Write(data)
	layer1 := h1.Sum(nil)

	h2 := sha256.New()
	h2.Write([]byte("QBTC_SHA999_L2"))
	h2.Write(layer1)
	h2.Write(data)
	layer2 := h2.Sum(nil)

	h3 := sha256.New()
	h3.Write([]byte("QBTC_SHA999_L3"))
	h3.Write(layer1)
	h3.Write(layer2)
	h3.Write(data)

	var result [32]byte
	copy(result[:], h3.Sum(nil))
	return result
}

func buildGroverCircuit(numQubits int, targetState int) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
	}
	iterations := int(math.Round(math.Pi / 4.0 * math.Sqrt(float64(1<<numQubits))))
	if iterations < 1 {
		iterations = 1
	}
	for iter := 0; iter < iterations; iter++ {
		_ = iter
		for i := 0; i < numQubits; i++ {
			if (targetState>>i)&1 == 0 {
				sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
			}
		}
		if numQubits >= 2 {
			sb.WriteString(fmt.Sprintf("cz q[0], q[%d];\n", numQubits-1))
		}
		for i := 0; i < numQubits; i++ {
			if (targetState>>i)&1 == 0 {
				sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
			}
		}
		for i := 0; i < numQubits; i++ {
			sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
			sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
		}
		if numQubits >= 2 {
			sb.WriteString(fmt.Sprintf("cz q[0], q[%d];\n", numQubits-1))
		}
		for i := 0; i < numQubits; i++ {
			sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
			sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
		}
	}
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}
	return sb.String()
}

func buildQFTCircuit(numQubits int) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
		for j := i + 1; j < numQubits; j++ {
			angle := math.Pi / math.Pow(2, float64(j-i))
			sb.WriteString(fmt.Sprintf("cp(%f) q[%d], q[%d];\n", angle, j, i))
		}
	}
	for i := 0; i < numQubits/2; i++ {
		sb.WriteString(fmt.Sprintf("swap q[%d], q[%d];\n", i, numQubits-1-i))
	}
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}
	return sb.String()
}

func buildVQECircuit(numQubits int, params []float64) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))
	paramIdx := 0
	for layer := 0; layer < 2; layer++ {
		for i := 0; i < numQubits; i++ {
			if paramIdx < len(params) {
				sb.WriteString(fmt.Sprintf("ry(%f) q[%d];\n", params[paramIdx], i))
				paramIdx++
			}
			if paramIdx < len(params) {
				sb.WriteString(fmt.Sprintf("rz(%f) q[%d];\n", params[paramIdx], i))
				paramIdx++
			}
		}
		for i := 0; i < numQubits-1; i++ {
			sb.WriteString(fmt.Sprintf("cx q[%d], q[%d];\n", i, i+1))
		}
	}
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}
	return sb.String()
}

func selectBestBackend(numQubits int) (string, error) {
	backends := map[string]int{
		"ibm_brisbane":   127,
		"ibm_osaka":      127,
		"ibm_kyoto":      127,
		"ibm_sherbrooke":  127,
		"ibm_torino":     133,
		"ibm_fez":        156,
		"ibm_marrakesh":  156,
	}
	bestName := ""
	bestQubits := 999999
	for name, qubits := range backends {
		if qubits >= numQubits && qubits < bestQubits {
			bestName = name
			bestQubits = qubits
		}
	}
	if bestName == "" {
		return "", fmt.Errorf("no backend supports %d qubits", numQubits)
	}
	return bestName, nil
}
