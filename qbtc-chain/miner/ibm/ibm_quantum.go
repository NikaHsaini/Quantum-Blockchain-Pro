// Package ibm implements the IBM Quantum mining backend for the QUBITCOIN network.
//
// This module provides a production-grade integration with IBM Quantum's Qiskit Runtime
// REST API, enabling QUBITCOIN miners to execute quantum circuits on real IBM quantum
// processors (QPUs) including Eagle r3 (127 qubits) and Heron r2 (156 qubits).
//
// Architecture:
//
//	┌──────────────────────────────────────────────────────────────────┐
//	│                    QUBITCOIN Mining Node                         │
//	│                                                                  │
//	│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────┐  │
//	│  │  QMaaS Job   │───▶│ IBM Quantum  │───▶│ Qiskit Runtime     │  │
//	│  │  Dispatcher  │    │ Client       │    │ REST API           │  │
//	│  └─────────────┘    └──────────────┘    └────────────────────┘  │
//	│         │                   │                     │              │
//	│         ▼                   ▼                     ▼              │
//	│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────┐  │
//	│  │  Result      │◀──│ Job Poller   │◀──│ IBM QPU            │  │
//	│  │  Verifier    │    │              │    │ (Eagle/Heron)      │  │
//	│  └─────────────┘    └──────────────┘    └────────────────────┘  │
//	└──────────────────────────────────────────────────────────────────┘
//
// Supported IBM QPU Backends (2025-2026):
//   - ibm_brisbane  (127 qubits, Eagle r3)
//   - ibm_osaka     (127 qubits, Eagle r3)
//   - ibm_kyoto     (127 qubits, Eagle r3)
//   - ibm_sherbrooke(127 qubits, Eagle r3)
//   - ibm_torino    (133 qubits, Heron r2)
//   - ibm_fez       (156 qubits, Heron r2)
//   - ibm_marrakesh (156 qubits, Heron r2)
//
// Authentication:
//   Uses IBM Cloud IAM bearer tokens, generated from an API key.
//   Tokens are cached and automatically refreshed before expiry.
//
// References:
//   - IBM Qiskit Runtime REST API: https://quantum.cloud.ibm.com/docs/api/qiskit-runtime-rest
//   - IBM Quantum Platform: https://quantum.cloud.ibm.com
//   - Qiskit SDK: https://www.ibm.com/quantum/qiskit
package ibm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Constants
// ============================================================

const (
	// IBM Cloud IAM token endpoint
	IAMTokenURL = "https://iam.cloud.ibm.com/identity/token"

	// IBM Quantum Runtime API base URLs
	IBMQuantumBaseURL   = "https://quantum.cloud.ibm.com/api/v1"
	IBMQuantumEUBaseURL = "https://eu-de.quantum.cloud.ibm.com/api/v1"

	// API version
	IBMAPIVersion = "2026-02-15"

	// Default timeouts
	DefaultHTTPTimeout     = 30 * time.Second
	DefaultJobPollInterval = 5 * time.Second
	DefaultJobTimeout      = 30 * time.Minute

	// Token refresh buffer (refresh 5 min before expiry)
	TokenRefreshBuffer = 5 * time.Minute
)

// ============================================================
// Backend Definitions
// ============================================================

// QPUBackend represents an IBM Quantum Processing Unit.
type QPUBackend struct {
	Name       string // e.g., "ibm_brisbane"
	Processor  string // e.g., "Eagle r3"
	NumQubits  int    // e.g., 127
	Region     string // "us" or "eu-de"
	Generation int    // Processor generation (3 for Eagle r3, 2 for Heron r2)
}

// AvailableBackends lists all supported IBM QPU backends.
var AvailableBackends = map[string]QPUBackend{
	"ibm_brisbane": {
		Name: "ibm_brisbane", Processor: "Eagle r3",
		NumQubits: 127, Region: "us", Generation: 3,
	},
	"ibm_osaka": {
		Name: "ibm_osaka", Processor: "Eagle r3",
		NumQubits: 127, Region: "us", Generation: 3,
	},
	"ibm_kyoto": {
		Name: "ibm_kyoto", Processor: "Eagle r3",
		NumQubits: 127, Region: "us", Generation: 3,
	},
	"ibm_sherbrooke": {
		Name: "ibm_sherbrooke", Processor: "Eagle r3",
		NumQubits: 127, Region: "us", Generation: 3,
	},
	"ibm_torino": {
		Name: "ibm_torino", Processor: "Heron r2",
		NumQubits: 133, Region: "us", Generation: 2,
	},
	"ibm_fez": {
		Name: "ibm_fez", Processor: "Heron r2",
		NumQubits: 156, Region: "us", Generation: 2,
	},
	"ibm_marrakesh": {
		Name: "ibm_marrakesh", Processor: "Heron r2",
		NumQubits: 156, Region: "us", Generation: 2,
	},
}

// ============================================================
// Errors
// ============================================================

var (
	ErrInvalidAPIKey       = errors.New("ibm: invalid API key")
	ErrTokenExpired        = errors.New("ibm: IAM token expired and refresh failed")
	ErrBackendNotFound     = errors.New("ibm: QPU backend not found")
	ErrBackendUnavailable  = errors.New("ibm: QPU backend is currently unavailable")
	ErrJobFailed           = errors.New("ibm: quantum job failed on QPU")
	ErrJobTimeout          = errors.New("ibm: quantum job timed out")
	ErrInvalidCircuit      = errors.New("ibm: invalid OpenQASM 3.0 circuit")
	ErrQubitLimitExceeded  = errors.New("ibm: circuit exceeds QPU qubit limit")
	ErrNoInstanceCRN       = errors.New("ibm: instance CRN not configured")
	ErrHTTPRequestFailed   = errors.New("ibm: HTTP request to IBM Quantum failed")
	ErrInvalidPrimitive    = errors.New("ibm: invalid primitive (must be 'sampler' or 'estimator')")
)

// ============================================================
// IAM Token Management
// ============================================================

// IAMToken represents an IBM Cloud IAM bearer token.
type IAMToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	Expiration  int64     `json:"expiration"`
	FetchedAt   time.Time `json:"-"`
}

// IsExpired checks if the token is expired or about to expire.
func (t *IAMToken) IsExpired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	expiryTime := t.FetchedAt.Add(time.Duration(t.ExpiresIn) * time.Second)
	return time.Now().After(expiryTime.Add(-TokenRefreshBuffer))
}

// ============================================================
// IBM Quantum Client
// ============================================================

// Config holds the configuration for the IBM Quantum client.
type Config struct {
	// APIKey is the IBM Cloud API key for authentication.
	APIKey string

	// InstanceCRN is the Cloud Resource Name of the IBM Quantum instance.
	InstanceCRN string

	// Region specifies the IBM Quantum region ("us" or "eu-de").
	Region string

	// PreferredBackend is the default QPU backend to use.
	PreferredBackend string

	// HTTPTimeout is the timeout for HTTP requests.
	HTTPTimeout time.Duration

	// JobPollInterval is the interval between job status polls.
	JobPollInterval time.Duration

	// JobTimeout is the maximum time to wait for a job to complete.
	JobTimeout time.Duration

	// ResilienceLevel is the Qiskit Runtime resilience level (0, 1, or 2).
	// 0: No error mitigation
	// 1: M3 (Matrix-free Measurement Mitigation)
	// 2: ZNE + PEC (Zero Noise Extrapolation + Probabilistic Error Cancellation)
	ResilienceLevel int

	// DynamicalDecoupling enables dynamical decoupling for error suppression.
	DynamicalDecoupling bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Region:              "us",
		PreferredBackend:    "ibm_brisbane",
		HTTPTimeout:         DefaultHTTPTimeout,
		JobPollInterval:     DefaultJobPollInterval,
		JobTimeout:          DefaultJobTimeout,
		ResilienceLevel:     1,
		DynamicalDecoupling: true,
	}
}

// Client is the IBM Quantum Runtime API client.
type Client struct {
	config     Config
	httpClient *http.Client
	token      *IAMToken
	tokenMu    sync.RWMutex

	// Metrics
	jobsSubmitted  int64
	jobsCompleted  int64
	jobsFailed     int64
	totalQPUTimeMs int64
	metricsMu      sync.Mutex
}

// NewClient creates a new IBM Quantum client.
func NewClient(config Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, ErrInvalidAPIKey
	}
	if config.InstanceCRN == "" {
		return nil, ErrNoInstanceCRN
	}

	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = DefaultHTTPTimeout
	}
	if config.JobPollInterval == 0 {
		config.JobPollInterval = DefaultJobPollInterval
	}
	if config.JobTimeout == 0 {
		config.JobTimeout = DefaultJobTimeout
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
	}

	// Pre-fetch the IAM token
	if err := client.refreshToken(); err != nil {
		return nil, fmt.Errorf("ibm: failed to authenticate: %w", err)
	}

	return client, nil
}

// baseURL returns the correct API base URL for the configured region.
func (c *Client) baseURL() string {
	if c.config.Region == "eu-de" {
		return IBMQuantumEUBaseURL
	}
	return IBMQuantumBaseURL
}

// ============================================================
// Token Management
// ============================================================

// refreshToken fetches a new IAM bearer token from IBM Cloud.
func (c *Client) refreshToken() error {
	body := fmt.Sprintf(
		"grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=%s",
		c.config.APIKey,
	)

	req, err := http.NewRequest("POST", IAMTokenURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("ibm: failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ibm: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ibm: IAM token request returned %d: %s", resp.StatusCode, string(respBody))
	}

	var token IAMToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("ibm: failed to decode IAM token: %w", err)
	}
	token.FetchedAt = time.Now()

	c.tokenMu.Lock()
	c.token = &token
	c.tokenMu.Unlock()

	return nil
}

// getToken returns a valid IAM token, refreshing if necessary.
func (c *Client) getToken() (string, error) {
	c.tokenMu.RLock()
	token := c.token
	c.tokenMu.RUnlock()

	if token.IsExpired() {
		if err := c.refreshToken(); err != nil {
			return "", ErrTokenExpired
		}
		c.tokenMu.RLock()
		token = c.token
		c.tokenMu.RUnlock()
	}

	return token.AccessToken, nil
}

// ============================================================
// API Request Helpers
// ============================================================

// doRequest executes an authenticated HTTP request to the IBM Quantum API.
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ibm: failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	url := c.baseURL() + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("ibm: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Service-CRN", c.config.InstanceCRN)
	req.Header.Set("IBM-API-Version", IBMAPIVersion)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}

	return resp, nil
}

// ============================================================
// Backend Discovery
// ============================================================

// BackendStatus represents the status of an IBM QPU backend.
type BackendStatus struct {
	Name          string `json:"backend_name"`
	NumQubits     int    `json:"num_qubits"`
	Status        string `json:"status"` // "active", "maintenance", "retired"
	Processor     string `json:"processor_type"`
	PendingJobs   int    `json:"pending_jobs"`
	MaxExperiments int   `json:"max_experiments"`
}

// ListBackends returns the list of available QPU backends.
func (c *Client) ListBackends() ([]BackendStatus, error) {
	resp, err := c.doRequest("GET", "/backends", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ibm: list backends returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Backends []BackendStatus `json:"backends"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ibm: failed to decode backends: %w", err)
	}

	return result.Backends, nil
}

// SelectBestBackend selects the best available backend for a given qubit count.
func (c *Client) SelectBestBackend(numQubits int) (string, error) {
	// First, try the preferred backend
	if backend, ok := AvailableBackends[c.config.PreferredBackend]; ok {
		if backend.NumQubits >= numQubits {
			return backend.Name, nil
		}
	}

	// Find the smallest backend that can handle the circuit
	var bestBackend string
	bestQubits := math.MaxInt32

	for name, backend := range AvailableBackends {
		if backend.NumQubits >= numQubits && backend.NumQubits < bestQubits {
			bestBackend = name
			bestQubits = backend.NumQubits
		}
	}

	if bestBackend == "" {
		return "", fmt.Errorf("%w: need %d qubits, max available is 156", ErrQubitLimitExceeded, numQubits)
	}

	return bestBackend, nil
}

// ============================================================
// Job Submission & Execution
// ============================================================

// Primitive represents a Qiskit Runtime primitive.
type Primitive string

const (
	PrimitiveSampler   Primitive = "sampler"
	PrimitiveEstimator Primitive = "estimator"
)

// JobRequest represents a request to submit a quantum job.
type JobRequest struct {
	// ProgramID is the primitive to use ("sampler" or "estimator").
	ProgramID Primitive `json:"program_id"`

	// Backend is the target QPU backend name.
	Backend string `json:"backend"`

	// SessionID is the optional session ID for session-based execution.
	SessionID string `json:"session_id,omitempty"`

	// Params contains the job parameters.
	Params JobParams `json:"params"`
}

// JobParams contains the parameters for a quantum job.
type JobParams struct {
	// Pubs is the list of Primitive Unified Blocs (circuits + observables).
	Pubs [][]interface{} `json:"pubs"`

	// Options contains execution options.
	Options JobOptions `json:"options"`

	// Version is the primitive version (always 2).
	Version int `json:"version"`

	// ResilienceLevel is the error mitigation level (0, 1, or 2).
	ResilienceLevel int `json:"resilience_level,omitempty"`
}

// JobOptions contains execution options for a quantum job.
type JobOptions struct {
	DynamicalDecoupling *DDOptions `json:"dynamical_decoupling,omitempty"`
	Shots               int        `json:"default_shots,omitempty"`
}

// DDOptions contains dynamical decoupling options.
type DDOptions struct {
	Enable bool `json:"enable"`
}

// JobResponse represents the response from submitting a job.
type JobResponse struct {
	ID        string `json:"id"`
	Backend   string `json:"backend"`
	Status    string `json:"status"`
	CreatedAt string `json:"created"`
}

// JobResult represents the result of a completed quantum job.
type JobResult struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"` // "Queued", "Running", "Completed", "Failed", "Cancelled"
	Backend   string                 `json:"backend"`
	Results   map[string]interface{} `json:"results"`
	Metrics   map[string]interface{} `json:"metrics"`
	CreatedAt string                 `json:"created"`
	EndedAt   string                 `json:"ended"`
}

// SubmitSamplerJob submits a sampler job with one or more OpenQASM 3.0 circuits.
func (c *Client) SubmitSamplerJob(circuits []string, backend string, shots int) (*JobResponse, error) {
	if backend == "" {
		backend = c.config.PreferredBackend
	}

	// Validate backend
	if _, ok := AvailableBackends[backend]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrBackendNotFound, backend)
	}

	// Build PUBs (each circuit is a PUB)
	pubs := make([][]interface{}, len(circuits))
	for i, circuit := range circuits {
		pubs[i] = []interface{}{circuit}
	}

	jobReq := JobRequest{
		ProgramID: PrimitiveSampler,
		Backend:   backend,
		Params: JobParams{
			Pubs:    pubs,
			Version: 2,
			Options: JobOptions{
				Shots: shots,
			},
			ResilienceLevel: c.config.ResilienceLevel,
		},
	}

	if c.config.DynamicalDecoupling {
		jobReq.Params.Options.DynamicalDecoupling = &DDOptions{Enable: true}
	}

	resp, err := c.doRequest("POST", "/jobs", jobReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ibm: submit job returned %d: %s", resp.StatusCode, string(respBody))
	}

	var jobResp JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return nil, fmt.Errorf("ibm: failed to decode job response: %w", err)
	}

	c.metricsMu.Lock()
	c.jobsSubmitted++
	c.metricsMu.Unlock()

	return &jobResp, nil
}

// SubmitEstimatorJob submits an estimator job with circuits and observables.
func (c *Client) SubmitEstimatorJob(circuits []string, observables []string, backend string) (*JobResponse, error) {
	if backend == "" {
		backend = c.config.PreferredBackend
	}

	if _, ok := AvailableBackends[backend]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrBackendNotFound, backend)
	}

	if len(circuits) != len(observables) {
		return nil, fmt.Errorf("ibm: circuits and observables must have the same length")
	}

	pubs := make([][]interface{}, len(circuits))
	for i := range circuits {
		pubs[i] = []interface{}{circuits[i], observables[i]}
	}

	jobReq := JobRequest{
		ProgramID: PrimitiveEstimator,
		Backend:   backend,
		Params: JobParams{
			Pubs:            pubs,
			Version:         2,
			ResilienceLevel: c.config.ResilienceLevel,
		},
	}

	if c.config.DynamicalDecoupling {
		jobReq.Params.Options.DynamicalDecoupling = &DDOptions{Enable: true}
	}

	resp, err := c.doRequest("POST", "/jobs", jobReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ibm: submit estimator job returned %d: %s", resp.StatusCode, string(respBody))
	}

	var jobResp JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return nil, fmt.Errorf("ibm: failed to decode job response: %w", err)
	}

	c.metricsMu.Lock()
	c.jobsSubmitted++
	c.metricsMu.Unlock()

	return &jobResp, nil
}

// GetJobStatus returns the current status and results of a job.
func (c *Client) GetJobStatus(jobID string) (*JobResult, error) {
	resp, err := c.doRequest("GET", "/jobs/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ibm: get job status returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result JobResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ibm: failed to decode job result: %w", err)
	}

	return &result, nil
}

// WaitForJob polls the job status until it completes or times out.
func (c *Client) WaitForJob(jobID string) (*JobResult, error) {
	deadline := time.Now().Add(c.config.JobTimeout)

	for time.Now().Before(deadline) {
		result, err := c.GetJobStatus(jobID)
		if err != nil {
			return nil, err
		}

		switch result.Status {
		case "Completed":
			c.metricsMu.Lock()
			c.jobsCompleted++
			c.metricsMu.Unlock()
			return result, nil

		case "Failed", "Cancelled":
			c.metricsMu.Lock()
			c.jobsFailed++
			c.metricsMu.Unlock()
			return result, fmt.Errorf("%w: status=%s", ErrJobFailed, result.Status)

		default:
			// "Queued", "Running" — keep polling
			time.Sleep(c.config.JobPollInterval)
		}
	}

	return nil, ErrJobTimeout
}

// ============================================================
// Session Management
// ============================================================

// SessionResponse represents a session creation response.
type SessionResponse struct {
	ID string `json:"id"`
}

// CreateSession creates a new Qiskit Runtime session.
func (c *Client) CreateSession(mode string, maxTTL int) (*SessionResponse, error) {
	if mode == "" {
		mode = "dedicated"
	}
	if maxTTL == 0 {
		maxTTL = 28800 // 8 hours
	}

	body := map[string]interface{}{
		"mode":    mode,
		"max_ttl": maxTTL,
	}

	resp, err := c.doRequest("POST", "/sessions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ibm: create session returned %d: %s", resp.StatusCode, string(respBody))
	}

	var session SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("ibm: failed to decode session response: %w", err)
	}

	return &session, nil
}

// ============================================================
// QUBITCOIN Mining Integration
// ============================================================

// MiningCircuit represents a quantum circuit submitted for mining.
type MiningCircuit struct {
	// JobID is the on-chain QMaaS job ID.
	JobID [32]byte

	// OpenQASM is the OpenQASM 3.0 circuit string.
	OpenQASM string

	// NumQubits is the number of qubits in the circuit.
	NumQubits int

	// Shots is the number of measurement shots.
	Shots int

	// Backend is the target IBM QPU backend.
	Backend string

	// ResilienceLevel is the error mitigation level.
	ResilienceLevel int
}

// MiningResult represents the result of mining a quantum circuit.
type MiningResult struct {
	// JobID is the on-chain QMaaS job ID.
	JobID [32]byte

	// IBMJobID is the IBM Quantum job ID.
	IBMJobID string

	// ResultHash is the SHA-256 hash of the result data.
	ResultHash [32]byte

	// RawResults is the raw result data from IBM Quantum.
	RawResults map[string]interface{}

	// Metrics contains execution metrics (QPU time, etc.)
	Metrics map[string]interface{}

	// Backend is the QPU backend that executed the circuit.
	Backend string

	// ExecutionTimeMs is the total execution time in milliseconds.
	ExecutionTimeMs int64
}

// MineCircuit executes a quantum circuit on an IBM QPU and returns the result.
// This is the main entry point for QUBITCOIN miners using IBM Quantum hardware.
func (c *Client) MineCircuit(circuit MiningCircuit) (*MiningResult, error) {
	startTime := time.Now()

	// Select backend if not specified
	backend := circuit.Backend
	if backend == "" {
		var err error
		backend, err = c.SelectBestBackend(circuit.NumQubits)
		if err != nil {
			return nil, err
		}
	}

	// Validate circuit
	if circuit.OpenQASM == "" {
		return nil, ErrInvalidCircuit
	}
	if circuit.Shots == 0 {
		circuit.Shots = 4096 // Default shots
	}

	// Submit the job
	jobResp, err := c.SubmitSamplerJob(
		[]string{circuit.OpenQASM},
		backend,
		circuit.Shots,
	)
	if err != nil {
		return nil, fmt.Errorf("ibm: failed to submit mining job: %w", err)
	}

	// Wait for completion
	jobResult, err := c.WaitForJob(jobResp.ID)
	if err != nil {
		return nil, fmt.Errorf("ibm: mining job failed: %w", err)
	}

	// Compute result hash
	resultJSON, _ := json.Marshal(jobResult.Results)
	resultHashBytes := sha256.Sum256(resultJSON)

	executionTime := time.Since(startTime).Milliseconds()

	c.metricsMu.Lock()
	c.totalQPUTimeMs += executionTime
	c.metricsMu.Unlock()

	return &MiningResult{
		JobID:           circuit.JobID,
		IBMJobID:        jobResp.ID,
		ResultHash:      resultHashBytes,
		RawResults:      jobResult.Results,
		Metrics:         jobResult.Metrics,
		Backend:         backend,
		ExecutionTimeMs: executionTime,
	}, nil
}

// ============================================================
// Quantum Circuit Builders (for common mining patterns)
// ============================================================

// BuildGroverCircuit generates an OpenQASM 3.0 circuit for Grover's algorithm.
func BuildGroverCircuit(numQubits int, targetState int) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))

	// Initialize superposition
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
	}

	// Grover iterations (optimal: pi/4 * sqrt(2^n))
	iterations := int(math.Round(math.Pi / 4.0 * math.Sqrt(float64(1<<numQubits))))
	if iterations < 1 {
		iterations = 1
	}

	for iter := 0; iter < iterations; iter++ {
		// Oracle: flip the target state
		for i := 0; i < numQubits; i++ {
			if (targetState>>i)&1 == 0 {
				sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
			}
		}
		// Multi-controlled Z (simplified as CZ chain)
		if numQubits >= 2 {
			sb.WriteString(fmt.Sprintf("cz q[0], q[%d];\n", numQubits-1))
		}
		for i := 0; i < numQubits; i++ {
			if (targetState>>i)&1 == 0 {
				sb.WriteString(fmt.Sprintf("x q[%d];\n", i))
			}
		}

		// Diffusion operator
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

	// Measure all qubits
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}

	return sb.String()
}

// BuildQFTCircuit generates an OpenQASM 3.0 circuit for the Quantum Fourier Transform.
func BuildQFTCircuit(numQubits int) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))

	// QFT circuit
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("h q[%d];\n", i))
		for j := i + 1; j < numQubits; j++ {
			angle := math.Pi / math.Pow(2, float64(j-i))
			sb.WriteString(fmt.Sprintf("cp(%f) q[%d], q[%d];\n", angle, j, i))
		}
	}

	// Swap qubits for correct ordering
	for i := 0; i < numQubits/2; i++ {
		sb.WriteString(fmt.Sprintf("swap q[%d], q[%d];\n", i, numQubits-1-i))
	}

	// Measure
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}

	return sb.String()
}

// BuildVQECircuit generates an OpenQASM 3.0 circuit for VQE (Variational Quantum Eigensolver).
func BuildVQECircuit(numQubits int, params []float64) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")
	sb.WriteString("include \"stdgates.inc\";\n")
	sb.WriteString(fmt.Sprintf("qubit[%d] q;\n", numQubits))
	sb.WriteString(fmt.Sprintf("bit[%d] c;\n", numQubits))

	// Hardware-efficient ansatz
	paramIdx := 0
	for layer := 0; layer < 2; layer++ {
		// Rotation layer
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
		// Entangling layer
		for i := 0; i < numQubits-1; i++ {
			sb.WriteString(fmt.Sprintf("cx q[%d], q[%d];\n", i, i+1))
		}
	}

	// Measure
	for i := 0; i < numQubits; i++ {
		sb.WriteString(fmt.Sprintf("c[%d] = measure q[%d];\n", i, i))
	}

	return sb.String()
}

// ============================================================
// Metrics & Monitoring
// ============================================================

// Metrics returns the current mining metrics.
type Metrics struct {
	JobsSubmitted  int64 `json:"jobs_submitted"`
	JobsCompleted  int64 `json:"jobs_completed"`
	JobsFailed     int64 `json:"jobs_failed"`
	TotalQPUTimeMs int64 `json:"total_qpu_time_ms"`
	SuccessRate    float64 `json:"success_rate"`
}

// GetMetrics returns the current mining metrics.
func (c *Client) GetMetrics() Metrics {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	var successRate float64
	if c.jobsSubmitted > 0 {
		successRate = float64(c.jobsCompleted) / float64(c.jobsSubmitted) * 100.0
	}

	return Metrics{
		JobsSubmitted:  c.jobsSubmitted,
		JobsCompleted:  c.jobsCompleted,
		JobsFailed:     c.jobsFailed,
		TotalQPUTimeMs: c.totalQPUTimeMs,
		SuccessRate:    successRate,
	}
}

// ============================================================
// Utility Functions
// ============================================================

// HashResult computes the SHA-256 hash of a mining result for on-chain submission.
func HashResult(result *MiningResult) [32]byte {
	data, _ := json.Marshal(result.RawResults)
	return sha256.Sum256(data)
}

// ResultToHex converts a result hash to a hex string for on-chain submission.
func ResultToHex(hash [32]byte) string {
	return "0x" + hex.EncodeToString(hash[:])
}
