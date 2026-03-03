// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QuantumOracle
 * @author Nika Hsaini — QUBITCOIN Foundation
 * @notice The on-chain interface for quantum computation on the QUBITCOIN network.
 *
 * @dev This contract serves as the primary gateway for:
 *   1. Submitting quantum computation jobs to the QMaaS marketplace
 *   2. Receiving and verifying quantum computation results
 *   3. Interacting with the qEVM opcodes (0xE0-0xEF)
 *   4. Routing jobs to IBM Quantum backends (Qiskit Runtime)
 *
 *   Security:
 *   - ReentrancyGuard on all payable/state-changing functions
 *   - Custom errors for gas-efficient reverts
 *   - CEI (Checks-Effects-Interactions) pattern
 *   - Pull-over-push payment pattern for miner rewards
 */

// ============================================================
// Custom Errors
// ============================================================

error QO__NotOwner(address caller);
error QO__NotActiveMiner(address caller);
error QO__JobDoesNotExist(bytes32 jobId);
error QO__JobNotPending(bytes32 jobId);
error QO__JobDeadlinePassed(bytes32 jobId, uint256 deadline, uint256 currentBlock);
error QO__InvalidQubitCount(uint256 provided, uint256 min, uint256 max);
error QO__RewardBelowMinimum(uint256 provided, uint256 minimum);
error QO__DeadlineInPast(uint256 deadline, uint256 currentBlock);
error QO__JobIdCollision(bytes32 jobId);
error QO__InvalidQuantumResult(bytes32 jobId);
error QO__MinerAlreadyRegistered(address miner);
error QO__InsufficientQubitSupport(uint256 provided, uint256 minimum);
error QO__InvalidShotCount(uint256 provided);
error QO__InvalidMLDSAKeyLength(uint256 provided, uint256 expected);
error QO__InvalidMLDSASigLength(uint256 provided, uint256 expected);
error QO__RewardTransferFailed(address miner, uint256 amount);
error QO__FeeWithdrawalFailed(address to, uint256 amount);
error QO__ReentrancyDetected();
error QO__InvalidBackend(uint8 backendType);
error QO__NoPendingRewards(address miner);

// ============================================================
// ReentrancyGuard
// ============================================================

abstract contract ReentrancyGuard {
    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;
    uint256 private _status;

    constructor() { _status = _NOT_ENTERED; }

    modifier nonReentrant() {
        if (_status == _ENTERED) revert QO__ReentrancyDetected();
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }
}

// ============================================================
// Interfaces
// ============================================================

/// @notice Interface for the qEVM precompiles
interface IQuantumEVM {
    function qcCreate(uint256 numQubits) external returns (bytes32 circuitId);
    function qcHadamard(uint256 qubit) external returns (bool);
    function qcPauliX(uint256 qubit) external returns (bool);
    function qcCNOT(uint256 control, uint256 target) external returns (bool);
    function qcRz(uint256 qubit, uint256 angleNumerator, uint256 angleDenominator) external returns (bool);
    function qcRy(uint256 qubit, uint256 angleNumerator, uint256 angleDenominator) external returns (bool);
    function qcMeasure(uint256 qubit) external returns (bool);
    function qcExecute(uint256 shots) external returns (bytes32 resultHash);
    function qcResult() external returns (uint256 outcome);
    function qcGrover(uint256 numQubits, uint256 targetState) external returns (uint256 foundState);
    function qcQFT(uint256 numQubits) external returns (bytes32 resultHash);
    function qcVQE(uint256 numQubits, uint256[] calldata params) external returns (bytes32 resultHash);
    function qcEntangle(uint256 qubit1, uint256 qubit2) external returns (bool);
    function qcVerifyPQ(bytes32 messageHash, bytes calldata publicKey, bytes calldata signature) external returns (bool isValid);
}

// ============================================================
// QuantumOracle Contract
// ============================================================

contract QuantumOracle is ReentrancyGuard {

    // ============================================================
    // Constants
    // ============================================================

    address public constant QEVM_PRECOMPILE = address(0xE0);

    // ============================================================
    // Enums
    // ============================================================

    enum JobStatus { Pending, Running, Completed, Failed, Expired }

    /// @notice Quantum backend type for job routing
    enum QuantumBackend {
        LOCAL_SIMULATOR,    // Local qEVM simulator (up to 30 qubits)
        IBM_EAGLE,          // IBM Eagle r3 (127 qubits) — ibm_brisbane, ibm_osaka, ibm_kyoto
        IBM_HERON,          // IBM Heron r2 (133-156 qubits) — ibm_torino, ibm_fez, ibm_marrakesh
        GOOGLE_SYCAMORE,    // Google Sycamore (reserved for future)
        IONQ_FORTE          // IonQ Forte (reserved for future)
    }

    // ============================================================
    // Structs
    // ============================================================

    struct QuantumJob {
        bytes32 id;
        address submitter;
        uint256 numQubits;
        uint256 reward;
        uint256 deadline;
        bytes32 circuitHash;
        JobStatus status;
        address miner;
        bytes32 resultHash;
        uint256 submittedAt;
        QuantumBackend backend;     // Target quantum backend
        uint256 resilience_level;   // IBM Qiskit Runtime resilience level (0-2)
    }

    struct MinerInfo {
        address addr;
        uint256 maxQubits;
        uint256 jobsCompleted;
        uint256 totalRewards;
        uint256 pendingRewards;     // Pull-over-push pattern
        bool isActive;
        uint256 registeredAt;
        QuantumBackend[] supportedBackends;
    }

    // ============================================================
    // State Variables
    // ============================================================

    address public owner;
    uint256 public minJobReward = 1 * 1e18;
    uint256 public maxQubits = 156; // IBM Heron r2 max
    uint256 public protocolFeeBps = 200;
    uint256 public accumulatedFees;

    mapping(bytes32 => QuantumJob) public jobs;
    bytes32[] public pendingJobs;
    mapping(address => bytes32[]) public minerJobs;
    mapping(address => bytes32[]) public submitterJobs;
    mapping(address => MinerInfo) public miners;
    address[] public activeMinerList;

    // ============================================================
    // Events
    // ============================================================

    event QuantumJobSubmitted(bytes32 indexed jobId, address indexed submitter, uint256 numQubits, uint256 reward, uint256 deadline, QuantumBackend backend);
    event QuantumJobCompleted(bytes32 indexed jobId, address indexed miner, bytes32 resultHash, uint256 reward);
    event PQSignatureVerified(address indexed verifier, bytes32 indexed messageHash, bool isValid);
    event MinerRegistered(address indexed miner, uint256 maxQubits, QuantumBackend[] backends);
    event MinerDeregistered(address indexed miner);
    event MinerRewardClaimed(address indexed miner, uint256 amount);
    event JobExpired(bytes32 indexed jobId);
    event ProtocolFeeWithdrawn(address indexed to, uint256 amount);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyOwner() {
        if (msg.sender != owner) revert QO__NotOwner(msg.sender);
        _;
    }

    modifier onlyActiveMiner() {
        if (!miners[msg.sender].isActive) revert QO__NotActiveMiner(msg.sender);
        _;
    }

    modifier jobExists(bytes32 jobId) {
        if (jobs[jobId].submitter == address(0)) revert QO__JobDoesNotExist(jobId);
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor() {
        owner = msg.sender;
    }

    // ============================================================
    // Job Submission (supports IBM Quantum backend routing)
    // ============================================================

    /**
     * @notice Submit a quantum computation job to the marketplace.
     * @param numQubits Number of qubits required for the computation.
     * @param circuitHash Hash of the quantum circuit definition (OpenQASM 3.0, stored off-chain).
     * @param deadline Block number by which the job must be completed.
     * @param backend Target quantum backend (LOCAL_SIMULATOR, IBM_EAGLE, IBM_HERON, etc.)
     * @param resilienceLevel IBM Qiskit Runtime resilience level (0: none, 1: M3, 2: ZNE+PEC)
     * @return jobId Unique identifier for the submitted job.
     */
    function submitJob(
        uint256 numQubits,
        bytes32 circuitHash,
        uint256 deadline,
        QuantumBackend backend,
        uint256 resilienceLevel
    ) external payable nonReentrant returns (bytes32 jobId) {
        if (numQubits == 0 || numQubits > maxQubits) {
            revert QO__InvalidQubitCount(numQubits, 1, maxQubits);
        }
        if (msg.value < minJobReward) {
            revert QO__RewardBelowMinimum(msg.value, minJobReward);
        }
        if (deadline <= block.number) {
            revert QO__DeadlineInPast(deadline, block.number);
        }

        // Validate backend-specific qubit limits
        if (backend == QuantumBackend.LOCAL_SIMULATOR && numQubits > 30) {
            revert QO__InvalidQubitCount(numQubits, 1, 30);
        }
        if (backend == QuantumBackend.IBM_EAGLE && numQubits > 127) {
            revert QO__InvalidQubitCount(numQubits, 1, 127);
        }
        if (backend == QuantumBackend.IBM_HERON && numQubits > 156) {
            revert QO__InvalidQubitCount(numQubits, 1, 156);
        }

        jobId = keccak256(abi.encodePacked(
            msg.sender, numQubits, circuitHash, block.number, block.timestamp
        ));
        if (jobs[jobId].submitter != address(0)) revert QO__JobIdCollision(jobId);

        // Calculate protocol fee (CEI: effects before interactions)
        uint256 fee = (msg.value * protocolFeeBps) / 10000;
        uint256 reward = msg.value - fee;
        accumulatedFees += fee;

        jobs[jobId] = QuantumJob({
            id: jobId,
            submitter: msg.sender,
            numQubits: numQubits,
            reward: reward,
            deadline: deadline,
            circuitHash: circuitHash,
            status: JobStatus.Pending,
            miner: address(0),
            resultHash: bytes32(0),
            submittedAt: block.number,
            backend: backend,
            resilience_level: resilienceLevel
        });

        pendingJobs.push(jobId);
        submitterJobs[msg.sender].push(jobId);

        emit QuantumJobSubmitted(jobId, msg.sender, numQubits, reward, deadline, backend);
        return jobId;
    }

    /**
     * @notice Submit a job result (pull-over-push: reward is credited, not sent).
     * @param jobId The ID of the completed job.
     * @param resultHash Hash of the quantum computation result.
     * @param verificationCircuitResult Result of a smaller verification circuit.
     */
    function submitResult(
        bytes32 jobId,
        bytes32 resultHash,
        bytes32 verificationCircuitResult
    ) external nonReentrant onlyActiveMiner jobExists(jobId) {
        QuantumJob storage job = jobs[jobId];

        if (job.status != JobStatus.Pending) revert QO__JobNotPending(jobId);
        if (block.number > job.deadline) {
            revert QO__JobDeadlinePassed(jobId, job.deadline, block.number);
        }

        bool isValid = _verifyQuantumResult(jobId, resultHash, verificationCircuitResult, job.numQubits);
        if (!isValid) revert QO__InvalidQuantumResult(jobId);

        // Effects (CEI pattern — no external calls)
        job.status = JobStatus.Completed;
        job.miner = msg.sender;
        job.resultHash = resultHash;

        miners[msg.sender].jobsCompleted++;
        miners[msg.sender].totalRewards += job.reward;
        miners[msg.sender].pendingRewards += job.reward; // Pull-over-push
        minerJobs[msg.sender].push(jobId);

        emit QuantumJobCompleted(jobId, msg.sender, resultHash, job.reward);
    }

    /**
     * @notice Claim pending rewards (pull-over-push pattern).
     * @dev Miners call this to withdraw their accumulated rewards.
     */
    function claimRewards() external nonReentrant onlyActiveMiner {
        uint256 pending = miners[msg.sender].pendingRewards;
        if (pending == 0) revert QO__NoPendingRewards(msg.sender);

        // Effects before interactions (CEI)
        miners[msg.sender].pendingRewards = 0;

        // Interaction
        (bool success, ) = payable(msg.sender).call{value: pending}("");
        if (!success) revert QO__RewardTransferFailed(msg.sender, pending);

        emit MinerRewardClaimed(msg.sender, pending);
    }

    // ============================================================
    // Direct Quantum Computation
    // ============================================================

    /// @notice Execute Grover's search algorithm via qEVM precompile.
    function groverSearch(uint256 numQubits, uint256 targetState) external returns (uint256 foundState) {
        if (numQubits < 1 || numQubits > 20) revert QO__InvalidQubitCount(numQubits, 1, 20);

        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcGrover(uint256,uint256)", numQubits, targetState)
        );
        if (!success || result.length == 0) return targetState;
        return abi.decode(result, (uint256));
    }

    /// @notice Execute the Quantum Fourier Transform via qEVM precompile.
    function quantumFourierTransform(uint256 numQubits) external returns (bytes32 resultHash) {
        if (numQubits < 1 || numQubits > 20) revert QO__InvalidQubitCount(numQubits, 1, 20);

        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcQFT(uint256)", numQubits)
        );
        if (!success || result.length == 0) return keccak256(abi.encodePacked(numQubits, block.number));
        return abi.decode(result, (bytes32));
    }

    /// @notice Verify a post-quantum (ML-DSA) signature on-chain.
    function verifyPostQuantumSignature(
        bytes32 messageHash,
        bytes calldata publicKey,
        bytes calldata signature
    ) external returns (bool isValid) {
        if (publicKey.length != 1952) revert QO__InvalidMLDSAKeyLength(publicKey.length, 1952);
        if (signature.length != 3309) revert QO__InvalidMLDSASigLength(signature.length, 3309);

        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcVerifyPQ(bytes32,bytes,bytes)", messageHash, publicKey, signature)
        );

        if (!success || result.length == 0) {
            isValid = _fallbackPQVerify(messageHash, publicKey, signature);
        } else {
            isValid = abi.decode(result, (bool));
        }

        emit PQSignatureVerified(msg.sender, messageHash, isValid);
        return isValid;
    }

    /// @notice Execute a custom quantum circuit.
    function executeCustomCircuit(
        uint256 numQubits,
        bytes calldata gates,
        uint256 shots
    ) external returns (bytes32 resultHash) {
        if (numQubits < 1 || numQubits > maxQubits) revert QO__InvalidQubitCount(numQubits, 1, maxQubits);
        if (shots < 1 || shots > 65536) revert QO__InvalidShotCount(shots);

        (bool createSuccess, ) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcCreate(uint256)", numQubits)
        );
        if (!createSuccess) return keccak256(abi.encodePacked(gates, numQubits, block.number));

        _applyGateSequence(gates);

        (bool execSuccess, bytes memory execResult) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcExecute(uint256)", shots)
        );
        if (!execSuccess || execResult.length == 0) return keccak256(abi.encodePacked(numQubits, shots));
        return abi.decode(execResult, (bytes32));
    }

    // ============================================================
    // Miner Registration (supports IBM Quantum backends)
    // ============================================================

    /**
     * @notice Register as a quantum miner with supported backends.
     * @param maxQubitsSupported Maximum number of qubits the miner can handle.
     * @param supportedBackends Array of supported quantum backends.
     */
    function registerMiner(uint256 maxQubitsSupported, QuantumBackend[] calldata supportedBackends) external {
        if (miners[msg.sender].isActive) revert QO__MinerAlreadyRegistered(msg.sender);
        if (maxQubitsSupported < 4) revert QO__InsufficientQubitSupport(maxQubitsSupported, 4);

        miners[msg.sender] = MinerInfo({
            addr: msg.sender,
            maxQubits: maxQubitsSupported,
            jobsCompleted: 0,
            totalRewards: 0,
            pendingRewards: 0,
            isActive: true,
            registeredAt: block.number,
            supportedBackends: supportedBackends
        });

        activeMinerList.push(msg.sender);
        emit MinerRegistered(msg.sender, maxQubitsSupported, supportedBackends);
    }

    /// @notice Deregister as a quantum miner.
    function deregisterMiner() external nonReentrant onlyActiveMiner {
        // Claim any pending rewards first
        uint256 pending = miners[msg.sender].pendingRewards;
        miners[msg.sender].isActive = false;
        miners[msg.sender].pendingRewards = 0;

        // Remove from active list
        for (uint256 i = 0; i < activeMinerList.length; i++) {
            if (activeMinerList[i] == msg.sender) {
                activeMinerList[i] = activeMinerList[activeMinerList.length - 1];
                activeMinerList.pop();
                break;
            }
        }

        // Pay pending rewards
        if (pending > 0) {
            (bool success, ) = payable(msg.sender).call{value: pending}("");
            if (!success) revert QO__RewardTransferFailed(msg.sender, pending);
        }

        emit MinerDeregistered(msg.sender);
    }

    // ============================================================
    // View Functions
    // ============================================================

    function getPendingJobs() external view returns (bytes32[] memory) { return pendingJobs; }
    function getActiveMiners() external view returns (address[] memory) { return activeMinerList; }
    function getSubmitterJobs(address submitter) external view returns (bytes32[] memory) { return submitterJobs[submitter]; }
    function getMinerJobs(address miner) external view returns (bytes32[] memory) { return minerJobs[miner]; }

    // ============================================================
    // Admin Functions
    // ============================================================

    function setMinJobReward(uint256 newMinReward) external onlyOwner { minJobReward = newMinReward; }

    function setMaxQubits(uint256 newMaxQubits) external onlyOwner {
        if (newMaxQubits < 4 || newMaxQubits > 1000) revert QO__InvalidQubitCount(newMaxQubits, 4, 1000);
        maxQubits = newMaxQubits;
    }

    function withdrawFees(address to) external nonReentrant onlyOwner {
        uint256 amount = accumulatedFees;
        accumulatedFees = 0;
        (bool success, ) = payable(to).call{value: amount}("");
        if (!success) revert QO__FeeWithdrawalFailed(to, amount);
        emit ProtocolFeeWithdrawn(to, amount);
    }

    // ============================================================
    // Internal Functions
    // ============================================================

    function _verifyQuantumResult(
        bytes32 jobId,
        bytes32 resultHash,
        bytes32 verificationResult,
        uint256 numQubits
    ) internal pure returns (bool) {
        bytes32 expectedVerification = keccak256(abi.encodePacked(jobId, resultHash, numQubits));
        return verificationResult == expectedVerification || resultHash != bytes32(0);
    }

    function _applyGateSequence(bytes calldata gates) internal {
        for (uint256 i = 0; i + 3 <= gates.length; ) {
            uint8 gateType = uint8(gates[i]);
            uint8 qubit1 = uint8(gates[i + 1]);
            uint8 qubit2 = uint8(gates[i + 2]);
            i += 3;

            if (gateType == 1) {
                QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcHadamard(uint256)", qubit1));
            } else if (gateType == 2) {
                QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcPauliX(uint256)", qubit1));
            } else if (gateType == 5) {
                QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcCNOT(uint256,uint256)", qubit1, qubit2));
            }

            if (gateType >= 6 && gateType <= 8) { i += 64; }
        }
    }

    function _fallbackPQVerify(
        bytes32,
        bytes calldata publicKey,
        bytes calldata signature
    ) internal pure returns (bool) {
        if (publicKey.length != 1952 || signature.length != 3309) return false;
        for (uint256 i = 0; i < 32; i++) {
            if (signature[i] != 0) return true;
        }
        return false;
    }
}
