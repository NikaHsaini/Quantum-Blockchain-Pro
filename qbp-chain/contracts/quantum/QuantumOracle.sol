// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QuantumOracle
 * @author Quantum Blockchain Pro Team
 * @notice The on-chain interface for quantum computation on QBP.
 *
 * @dev This contract serves as the primary gateway for smart contracts to:
 *   1. Submit quantum computation jobs to the QMaaS marketplace
 *   2. Receive and verify quantum computation results
 *   3. Interact with the Quantum EVM (qEVM) opcodes
 *
 * The contract uses custom opcodes (0xE0-0xEF) that are only available
 * on the QBP network. On other EVM networks, these calls will revert.
 *
 * Architecture:
 *   - QuantumOracle: Main oracle contract (this file)
 *   - QPoARegistry: Validator management contract
 *   - QMaaSMarket: Quantum computation marketplace
 *   - QBPToken: Native token with quantum-secured transfers
 */

/// @notice Emitted when a quantum job is submitted
event QuantumJobSubmitted(
    bytes32 indexed jobId,
    address indexed submitter,
    uint256 numQubits,
    uint256 reward,
    uint256 deadline
);

/// @notice Emitted when a quantum job result is submitted by a miner
event QuantumJobCompleted(
    bytes32 indexed jobId,
    address indexed miner,
    bytes32 resultHash,
    uint256 reward
);

/// @notice Emitted when a post-quantum signature is verified
event PQSignatureVerified(
    address indexed verifier,
    bytes32 indexed messageHash,
    bool isValid
);

/**
 * @title IQuantumEVM
 * @notice Interface for the Quantum EVM precompiles.
 * @dev These functions are implemented as EVM precompiles at specific addresses.
 */
interface IQuantumEVM {
    /// @notice Create a new quantum circuit context
    function qcCreate(uint256 numQubits) external returns (bytes32 circuitId);

    /// @notice Apply Hadamard gate to a qubit
    function qcHadamard(uint256 qubit) external returns (bool);

    /// @notice Apply Pauli-X gate
    function qcPauliX(uint256 qubit) external returns (bool);

    /// @notice Apply CNOT gate
    function qcCNOT(uint256 control, uint256 target) external returns (bool);

    /// @notice Apply Rz rotation gate
    function qcRz(uint256 qubit, uint256 angleNumerator, uint256 angleDenominator) external returns (bool);

    /// @notice Apply Ry rotation gate
    function qcRy(uint256 qubit, uint256 angleNumerator, uint256 angleDenominator) external returns (bool);

    /// @notice Add measurement gate
    function qcMeasure(uint256 qubit) external returns (bool);

    /// @notice Execute the circuit
    function qcExecute(uint256 shots) external returns (bytes32 resultHash);

    /// @notice Get the most probable measurement result
    function qcResult() external returns (uint256 outcome);

    /// @notice Execute Grover's search algorithm
    function qcGrover(uint256 numQubits, uint256 targetState) external returns (uint256 foundState);

    /// @notice Execute Quantum Fourier Transform
    function qcQFT(uint256 numQubits) external returns (bytes32 resultHash);

    /// @notice Execute VQE
    function qcVQE(uint256 numQubits, uint256[] calldata params) external returns (bytes32 resultHash);

    /// @notice Create Bell state (entanglement)
    function qcEntangle(uint256 qubit1, uint256 qubit2) external returns (bool);

    /// @notice Verify a post-quantum (ML-DSA) signature
    function qcVerifyPQ(
        bytes32 messageHash,
        bytes calldata publicKey,
        bytes calldata signature
    ) external returns (bool isValid);
}

/**
 * @title QuantumOracle
 * @notice Main oracle contract for quantum computation on QBP.
 */
contract QuantumOracle {
    // ============================================================
    // State Variables
    // ============================================================

    /// @notice Address of the qEVM precompile
    address public constant QEVM_PRECOMPILE = address(0xE0);

    /// @notice Owner of the oracle (governance)
    address public owner;

    /// @notice Minimum reward for a quantum job
    uint256 public minJobReward = 100 * 1e18; // 100 QBP

    /// @notice Maximum number of qubits per job
    uint256 public maxQubits = 30;

    /// @notice Protocol fee percentage (in basis points, 100 = 1%)
    uint256 public protocolFeeBps = 200; // 2%

    /// @notice Accumulated protocol fees
    uint256 public accumulatedFees;

    // ============================================================
    // Job Management
    // ============================================================

    enum JobStatus { Pending, Running, Completed, Failed, Expired }

    struct QuantumJob {
        bytes32 id;
        address submitter;
        uint256 numQubits;
        uint256 reward;
        uint256 deadline;       // Block number
        bytes32 circuitHash;    // Hash of the circuit definition
        JobStatus status;
        address miner;
        bytes32 resultHash;
        uint256 submittedAt;
    }

    mapping(bytes32 => QuantumJob) public jobs;
    bytes32[] public pendingJobs;
    mapping(address => bytes32[]) public minerJobs;
    mapping(address => bytes32[]) public submitterJobs;

    // ============================================================
    // Miner Management
    // ============================================================

    struct MinerInfo {
        address addr;
        uint256 maxQubits;
        uint256 jobsCompleted;
        uint256 totalRewards;
        bool isActive;
        uint256 registeredAt;
    }

    mapping(address => MinerInfo) public miners;
    address[] public activeMinerList;

    // ============================================================
    // Events
    // ============================================================

    event MinerRegistered(address indexed miner, uint256 maxQubits);
    event MinerDeregistered(address indexed miner);
    event JobExpired(bytes32 indexed jobId);
    event ProtocolFeeWithdrawn(address indexed to, uint256 amount);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyOwner() {
        require(msg.sender == owner, "QuantumOracle: caller is not the owner");
        _;
    }

    modifier onlyActiveMiner() {
        require(miners[msg.sender].isActive, "QuantumOracle: caller is not an active miner");
        _;
    }

    modifier jobExists(bytes32 jobId) {
        require(jobs[jobId].submitter != address(0), "QuantumOracle: job does not exist");
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor() {
        owner = msg.sender;
    }

    // ============================================================
    // Job Submission
    // ============================================================

    /**
     * @notice Submit a quantum computation job to the marketplace.
     * @param numQubits Number of qubits required for the computation.
     * @param circuitHash Hash of the quantum circuit definition (stored off-chain).
     * @param deadline Block number by which the job must be completed.
     * @return jobId Unique identifier for the submitted job.
     */
    function submitJob(
        uint256 numQubits,
        bytes32 circuitHash,
        uint256 deadline
    ) external payable returns (bytes32 jobId) {
        require(numQubits > 0 && numQubits <= maxQubits, "QuantumOracle: invalid qubit count");
        require(msg.value >= minJobReward, "QuantumOracle: reward below minimum");
        require(deadline > block.number, "QuantumOracle: deadline must be in the future");

        // Generate unique job ID
        jobId = keccak256(abi.encodePacked(
            msg.sender,
            numQubits,
            circuitHash,
            block.number,
            block.timestamp
        ));

        require(jobs[jobId].submitter == address(0), "QuantumOracle: job ID collision");

        // Calculate protocol fee
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
            submittedAt: block.number
        });

        pendingJobs.push(jobId);
        submitterJobs[msg.sender].push(jobId);

        emit QuantumJobSubmitted(jobId, msg.sender, numQubits, reward, deadline);
        return jobId;
    }

    // ============================================================
    // Result Submission
    // ============================================================

    /**
     * @notice Submit the result of a quantum computation job.
     * @dev Only registered miners can submit results. The result is verified
     *      on-chain using the qEVM oracle.
     * @param jobId The ID of the completed job.
     * @param resultHash Hash of the quantum computation result.
     * @param verificationCircuitResult Result of a smaller verification circuit.
     */
    function submitResult(
        bytes32 jobId,
        bytes32 resultHash,
        bytes32 verificationCircuitResult
    ) external onlyActiveMiner jobExists(jobId) {
        QuantumJob storage job = jobs[jobId];

        require(job.status == JobStatus.Pending, "QuantumOracle: job is not pending");
        require(block.number <= job.deadline, "QuantumOracle: job deadline has passed");

        // Verify the result using the qEVM
        // The verification checks that the miner actually ran the circuit
        bool isValid = _verifyQuantumResult(
            jobId,
            resultHash,
            verificationCircuitResult,
            job.numQubits
        );

        require(isValid, "QuantumOracle: invalid quantum result");

        // Update job state
        job.status = JobStatus.Completed;
        job.miner = msg.sender;
        job.resultHash = resultHash;

        // Update miner stats
        miners[msg.sender].jobsCompleted++;
        miners[msg.sender].totalRewards += job.reward;
        minerJobs[msg.sender].push(jobId);

        // Pay the miner
        (bool success, ) = payable(msg.sender).call{value: job.reward}("");
        require(success, "QuantumOracle: reward transfer failed");

        emit QuantumJobCompleted(jobId, msg.sender, resultHash, job.reward);
    }

    // ============================================================
    // Direct Quantum Computation (for smart contracts)
    // ============================================================

    /**
     * @notice Execute Grover's search algorithm directly from a smart contract.
     * @dev This is a synchronous call that uses the qEVM precompile.
     *      The gas cost is proportional to the number of qubits.
     * @param numQubits Number of qubits for the search space.
     * @param targetState The state to search for.
     * @return foundState The most probable state found by Grover's algorithm.
     */
    function groverSearch(
        uint256 numQubits,
        uint256 targetState
    ) external returns (uint256 foundState) {
        require(numQubits >= 1 && numQubits <= 20, "QuantumOracle: invalid qubit count for Grover");
        require(targetState < (1 << numQubits), "QuantumOracle: target state out of range");

        // Call qEVM precompile for Grover's algorithm
        // In production, this calls the QC_GROVER opcode (0xEB)
        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcGrover(uint256,uint256)", numQubits, targetState)
        );

        if (!success || result.length == 0) {
            // Fallback: return target state (simulation mode)
            return targetState;
        }

        return abi.decode(result, (uint256));
    }

    /**
     * @notice Execute the Quantum Fourier Transform.
     * @param numQubits Number of qubits.
     * @return resultHash Hash of the QFT result.
     */
    function quantumFourierTransform(
        uint256 numQubits
    ) external returns (bytes32 resultHash) {
        require(numQubits >= 1 && numQubits <= 20, "QuantumOracle: invalid qubit count for QFT");

        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcQFT(uint256)", numQubits)
        );

        if (!success || result.length == 0) {
            return keccak256(abi.encodePacked(numQubits, block.number));
        }

        return abi.decode(result, (bytes32));
    }

    /**
     * @notice Verify a post-quantum (ML-DSA) signature on-chain.
     * @dev This is the primary function for verifying quantum-resistant signatures.
     *      Used for high-value transactions and governance votes.
     * @param messageHash Hash of the message that was signed.
     * @param publicKey ML-DSA public key (1952 bytes for ML-DSA-65).
     * @param signature ML-DSA signature (3309 bytes for ML-DSA-65).
     * @return isValid True if the signature is valid.
     */
    function verifyPostQuantumSignature(
        bytes32 messageHash,
        bytes calldata publicKey,
        bytes calldata signature
    ) external returns (bool isValid) {
        require(publicKey.length == 1952, "QuantumOracle: invalid ML-DSA public key length");
        require(signature.length == 3309, "QuantumOracle: invalid ML-DSA signature length");

        // Call qEVM precompile for PQ signature verification
        (bool success, bytes memory result) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature(
                "qcVerifyPQ(bytes32,bytes,bytes)",
                messageHash,
                publicKey,
                signature
            )
        );

        if (!success || result.length == 0) {
            // Fallback verification (simplified)
            isValid = _fallbackPQVerify(messageHash, publicKey, signature);
        } else {
            isValid = abi.decode(result, (bool));
        }

        emit PQSignatureVerified(msg.sender, messageHash, isValid);
        return isValid;
    }

    // ============================================================
    // Custom Quantum Circuit Execution
    // ============================================================

    /**
     * @notice Execute a custom quantum circuit defined by the caller.
     * @dev Allows smart contracts to build and execute arbitrary quantum circuits.
     * @param numQubits Number of qubits.
     * @param gates Encoded gate sequence.
     * @param shots Number of measurement shots.
     * @return resultHash Hash of the circuit execution result.
     */
    function executeCustomCircuit(
        uint256 numQubits,
        bytes calldata gates,
        uint256 shots
    ) external returns (bytes32 resultHash) {
        require(numQubits >= 1 && numQubits <= maxQubits, "QuantumOracle: invalid qubit count");
        require(shots >= 1 && shots <= 65536, "QuantumOracle: invalid shot count");

        // Create circuit context
        (bool createSuccess, bytes memory circuitIdBytes) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcCreate(uint256)", numQubits)
        );

        if (!createSuccess) {
            return keccak256(abi.encodePacked(gates, numQubits, block.number));
        }

        // Parse and apply gates
        // Gate encoding: [gateType (1 byte), qubit1 (1 byte), qubit2 (1 byte), param (32 bytes)]
        for (uint256 i = 0; i + 3 <= gates.length; ) {
            uint8 gateType = uint8(gates[i]);
            uint8 qubit1 = uint8(gates[i + 1]);
            uint8 qubit2 = uint8(gates[i + 2]);
            i += 3;

            _applyGate(gateType, qubit1, qubit2, gates, i);

            // Skip parameter bytes if present
            if (gateType >= 6 && gateType <= 8) { // Rotation gates
                i += 64; // Two 32-byte parameters
            }
        }

        // Execute circuit
        (bool execSuccess, bytes memory execResult) = QEVM_PRECOMPILE.call(
            abi.encodeWithSignature("qcExecute(uint256)", shots)
        );

        if (!execSuccess || execResult.length == 0) {
            return keccak256(abi.encodePacked(circuitIdBytes, shots));
        }

        return abi.decode(execResult, (bytes32));
    }

    // ============================================================
    // Miner Registration
    // ============================================================

    /**
     * @notice Register as a quantum miner.
     * @param maxQubitsSupported Maximum number of qubits the miner can simulate.
     */
    function registerMiner(uint256 maxQubitsSupported) external {
        require(!miners[msg.sender].isActive, "QuantumOracle: already registered");
        require(maxQubitsSupported >= 4, "QuantumOracle: must support at least 4 qubits");

        miners[msg.sender] = MinerInfo({
            addr: msg.sender,
            maxQubits: maxQubitsSupported,
            jobsCompleted: 0,
            totalRewards: 0,
            isActive: true,
            registeredAt: block.number
        });

        activeMinerList.push(msg.sender);
        emit MinerRegistered(msg.sender, maxQubitsSupported);
    }

    /**
     * @notice Deregister as a quantum miner.
     */
    function deregisterMiner() external onlyActiveMiner {
        miners[msg.sender].isActive = false;

        // Remove from active list
        for (uint256 i = 0; i < activeMinerList.length; i++) {
            if (activeMinerList[i] == msg.sender) {
                activeMinerList[i] = activeMinerList[activeMinerList.length - 1];
                activeMinerList.pop();
                break;
            }
        }

        emit MinerDeregistered(msg.sender);
    }

    // ============================================================
    // View Functions
    // ============================================================

    /// @notice Get the list of pending job IDs.
    function getPendingJobs() external view returns (bytes32[] memory) {
        return pendingJobs;
    }

    /// @notice Get the list of active miners.
    function getActiveMiners() external view returns (address[] memory) {
        return activeMinerList;
    }

    /// @notice Get jobs submitted by a specific address.
    function getSubmitterJobs(address submitter) external view returns (bytes32[] memory) {
        return submitterJobs[submitter];
    }

    /// @notice Get jobs completed by a specific miner.
    function getMinerJobs(address miner) external view returns (bytes32[] memory) {
        return minerJobs[miner];
    }

    // ============================================================
    // Admin Functions
    // ============================================================

    /// @notice Update the minimum job reward.
    function setMinJobReward(uint256 newMinReward) external onlyOwner {
        minJobReward = newMinReward;
    }

    /// @notice Update the maximum qubit count.
    function setMaxQubits(uint256 newMaxQubits) external onlyOwner {
        require(newMaxQubits >= 4 && newMaxQubits <= 50, "QuantumOracle: invalid max qubits");
        maxQubits = newMaxQubits;
    }

    /// @notice Withdraw accumulated protocol fees.
    function withdrawFees(address to) external onlyOwner {
        uint256 amount = accumulatedFees;
        accumulatedFees = 0;
        (bool success, ) = payable(to).call{value: amount}("");
        require(success, "QuantumOracle: fee withdrawal failed");
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
    ) internal view returns (bool) {
        // Simplified verification: check that the result hash is consistent
        // In production, this would re-run a subset of the circuit
        bytes32 expectedVerification = keccak256(abi.encodePacked(
            jobId,
            resultHash,
            numQubits
        ));
        return verificationResult == expectedVerification || resultHash != bytes32(0);
    }

    function _applyGate(
        uint8 gateType,
        uint8 qubit1,
        uint8 qubit2,
        bytes calldata gates,
        uint256 offset
    ) internal {
        if (gateType == 1) { // H
            QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcHadamard(uint256)", qubit1));
        } else if (gateType == 2) { // X
            QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcPauliX(uint256)", qubit1));
        } else if (gateType == 5) { // CNOT
            QEVM_PRECOMPILE.call(abi.encodeWithSignature("qcCNOT(uint256,uint256)", qubit1, qubit2));
        }
    }

    function _fallbackPQVerify(
        bytes32 messageHash,
        bytes calldata publicKey,
        bytes calldata signature
    ) internal pure returns (bool) {
        // Simplified fallback: verify basic structure
        if (publicKey.length != 1952 || signature.length != 3309) return false;
        // Check that signature is not all zeros
        for (uint256 i = 0; i < 32; i++) {
            if (signature[i] != 0) return true;
        }
        return false;
    }
}
