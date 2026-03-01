// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QPoARegistry
 * @author Quantum Blockchain Pro Team
 * @notice Manages the set of authorized validators for the QPoA consensus.
 *
 * @dev Validators must:
 *   1. Stake a minimum amount of QBP tokens
 *   2. Register their ML-DSA public key (post-quantum)
 *   3. Prove quantum capability by solving periodic challenges
 *   4. Maintain uptime and performance standards
 *
 * Governance:
 *   - Adding validators requires a majority vote from existing validators
 *   - Removing validators requires a 2/3 supermajority
 *   - Quantum challenge failures result in slashing
 */

import "./QBPToken.sol";

contract QPoARegistry {
    // ============================================================
    // Constants
    // ============================================================

    /// @notice Minimum stake required to become a validator (in QBP wei)
    uint256 public constant MIN_STAKE = 10 * 1e18; // 10 QBP (calibré pour une offre de 21 000 tokens)

    /// @notice Maximum number of validators
    uint256 public constant MAX_VALIDATORS = 21;

    /// @notice Minimum number of validators
    uint256 public constant MIN_VALIDATORS = 3;

    /// @notice Slashing percentage for quantum challenge failure (in basis points)
    uint256 public constant SLASH_QUANTUM_FAIL_BPS = 500; // 5%

    /// @notice Slashing percentage for downtime (in basis points)
    uint256 public constant SLASH_DOWNTIME_BPS = 100; // 1%

    /// @notice ML-DSA public key size (FIPS 204 - ML-DSA-65)
    uint256 public constant MLDSA_PUBKEY_SIZE = 1952;

    // ============================================================
    // Data Structures
    // ============================================================

    enum ValidatorStatus {
        Inactive,
        Pending,    // Application submitted, awaiting vote
        Active,     // Authorized and active
        Slashed,    // Penalized but still active
        Jailed,     // Temporarily suspended
        Exiting     // Requested exit, waiting for unbonding period
    }

    struct Validator {
        address addr;
        bytes mldsaPublicKey;       // ML-DSA-65 public key (1952 bytes)
        uint256 stake;              // Staked QBP amount
        uint256 since;              // Block number when activated
        uint256 lastBlock;          // Last block signed
        uint256 blocksProduced;     // Total blocks produced
        uint256 quantumScore;       // Quantum challenge performance score
        uint256 slashCount;         // Number of times slashed
        ValidatorStatus status;
        bool quantumCapable;        // Has proven quantum capability
    }

    struct QuantumChallenge {
        uint256 id;
        uint256 issuedAt;           // Block number
        bytes32 circuitHash;        // Hash of the circuit to solve
        uint256 numQubits;
        bytes32 expectedResultHash; // Expected result (set after deadline)
        uint256 deadline;           // Block number deadline
        bool resolved;
    }

    struct Vote {
        address candidate;
        bool authorize;             // true = add, false = remove
        uint256 votes;
        mapping(address => bool) voted;
    }

    // ============================================================
    // State Variables
    // ============================================================

    QBPToken public immutable qbpToken;
    address public governance;

    mapping(address => Validator) public validators;
    address[] public validatorList;

    mapping(uint256 => QuantumChallenge) public challenges;
    uint256 public challengeCount;

    mapping(address => Vote) private votes;
    address[] public pendingVotes;

    mapping(address => uint256) public pendingWithdrawals;
    uint256 public unbondingPeriod = 50400; // ~7 days at 12s/block

    // ============================================================
    // Events
    // ============================================================

    event ValidatorApplicationSubmitted(address indexed validator, uint256 stake);
    event ValidatorActivated(address indexed validator, uint256 blockNumber);
    event ValidatorDeactivated(address indexed validator, string reason);
    event ValidatorSlashed(address indexed validator, uint256 amount, string reason);
    event ValidatorJailed(address indexed validator, uint256 until);
    event QuantumChallengeIssued(uint256 indexed challengeId, bytes32 circuitHash, uint256 deadline);
    event QuantumChallengeCompleted(uint256 indexed challengeId, address indexed validator, bool success);
    event VoteCast(address indexed voter, address indexed candidate, bool authorize);
    event StakeDeposited(address indexed validator, uint256 amount);
    event StakeWithdrawn(address indexed validator, uint256 amount);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyGovernance() {
        require(msg.sender == governance, "QPoARegistry: not governance");
        _;
    }

    modifier onlyActiveValidator() {
        require(
            validators[msg.sender].status == ValidatorStatus.Active ||
            validators[msg.sender].status == ValidatorStatus.Slashed,
            "QPoARegistry: not an active validator"
        );
        _;
    }

    modifier validatorExists(address addr) {
        require(validators[addr].addr != address(0), "QPoARegistry: validator not found");
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor(address _qbpToken, address _governance) {
        qbpToken = QBPToken(_qbpToken);
        governance = _governance;
    }

    // ============================================================
    // Validator Application
    // ============================================================

    /**
     * @notice Apply to become a validator.
     * @dev Requires staking MIN_STAKE QBP and providing an ML-DSA public key.
     * @param mldsaPublicKey The validator's ML-DSA-65 public key (1952 bytes).
     */
    function applyForValidator(bytes calldata mldsaPublicKey) external {
        require(mldsaPublicKey.length == MLDSA_PUBKEY_SIZE, "QPoARegistry: invalid ML-DSA key size");
        require(validators[msg.sender].addr == address(0), "QPoARegistry: already registered");
        require(validatorList.length < MAX_VALIDATORS, "QPoARegistry: maximum validators reached");

        // Transfer stake from applicant
        require(
            qbpToken.transferFrom(msg.sender, address(this), MIN_STAKE),
            "QPoARegistry: stake transfer failed"
        );

        validators[msg.sender] = Validator({
            addr: msg.sender,
            mldsaPublicKey: mldsaPublicKey,
            stake: MIN_STAKE,
            since: 0,
            lastBlock: 0,
            blocksProduced: 0,
            quantumScore: 0,
            slashCount: 0,
            status: ValidatorStatus.Pending,
            quantumCapable: false
        });

        emit ValidatorApplicationSubmitted(msg.sender, MIN_STAKE);
    }

    /**
     * @notice Add additional stake to strengthen validator position.
     * @param amount Additional QBP to stake.
     */
    function addStake(uint256 amount) external validatorExists(msg.sender) {
        require(amount > 0, "QPoARegistry: amount must be positive");
        require(
            qbpToken.transferFrom(msg.sender, address(this), amount),
            "QPoARegistry: stake transfer failed"
        );
        validators[msg.sender].stake += amount;
        emit StakeDeposited(msg.sender, amount);
    }

    // ============================================================
    // Governance Voting
    // ============================================================

    /**
     * @notice Vote to authorize or deauthorize a validator.
     * @param candidate Address of the validator candidate.
     * @param authorize True to authorize, false to deauthorize.
     */
    function castVote(address candidate, bool authorize) external onlyActiveValidator {
        require(candidate != msg.sender, "QPoARegistry: cannot vote for yourself");

        Vote storage vote = votes[candidate];
        require(!vote.voted[msg.sender], "QPoARegistry: already voted");

        if (vote.candidate == address(0)) {
            vote.candidate = candidate;
            vote.authorize = authorize;
            pendingVotes.push(candidate);
        }

        vote.voted[msg.sender] = true;
        vote.votes++;

        emit VoteCast(msg.sender, candidate, authorize);

        // Check if vote passed
        uint256 threshold = authorize
            ? validatorList.length / 2 + 1  // Simple majority for authorization
            : (validatorList.length * 2) / 3 + 1; // 2/3 supermajority for removal

        if (vote.votes >= threshold) {
            if (authorize) {
                _activateValidator(candidate);
            } else {
                _deactivateValidator(candidate, "Voted out by validators");
            }
            // Clear vote
            delete votes[candidate];
        }
    }

    // ============================================================
    // Quantum Challenge System
    // ============================================================

    /**
     * @notice Issue a quantum challenge to all validators.
     * @dev Only callable by the consensus engine (via system transaction).
     * @param circuitHash Hash of the quantum circuit to solve.
     * @param numQubits Number of qubits in the circuit.
     * @param deadline Block number deadline for submissions.
     */
    function issueQuantumChallenge(
        bytes32 circuitHash,
        uint256 numQubits,
        uint256 deadline
    ) external onlyGovernance returns (uint256 challengeId) {
        challengeId = ++challengeCount;

        challenges[challengeId] = QuantumChallenge({
            id: challengeId,
            issuedAt: block.number,
            circuitHash: circuitHash,
            numQubits: numQubits,
            expectedResultHash: bytes32(0),
            deadline: deadline,
            resolved: false
        });

        emit QuantumChallengeIssued(challengeId, circuitHash, deadline);
        return challengeId;
    }

    /**
     * @notice Submit a response to a quantum challenge.
     * @param challengeId The ID of the challenge.
     * @param resultHash Hash of the quantum circuit execution result.
     * @param mldsaSignature ML-DSA signature over the result hash.
     */
    function submitChallengeResponse(
        uint256 challengeId,
        bytes32 resultHash,
        bytes calldata mldsaSignature
    ) external onlyActiveValidator {
        QuantumChallenge storage challenge = challenges[challengeId];
        require(challenge.id != 0, "QPoARegistry: challenge not found");
        require(!challenge.resolved, "QPoARegistry: challenge already resolved");
        require(block.number <= challenge.deadline, "QPoARegistry: challenge deadline passed");

        // Verify the ML-DSA signature over the result
        // In production, this calls the qEVM precompile
        bool sigValid = _verifyMLDSASignature(
            resultHash,
            validators[msg.sender].mldsaPublicKey,
            mldsaSignature
        );

        require(sigValid, "QPoARegistry: invalid ML-DSA signature");

        // Update quantum score
        validators[msg.sender].quantumScore += 100;
        validators[msg.sender].quantumCapable = true;

        emit QuantumChallengeCompleted(challengeId, msg.sender, true);
    }

    /**
     * @notice Slash validators who failed to respond to a quantum challenge.
     * @param challengeId The ID of the expired challenge.
     * @param failedValidators List of validators who failed to respond.
     */
    function slashQuantumFailure(
        uint256 challengeId,
        address[] calldata failedValidators
    ) external onlyGovernance {
        QuantumChallenge storage challenge = challenges[challengeId];
        require(block.number > challenge.deadline, "QPoARegistry: challenge not yet expired");
        require(!challenge.resolved, "QPoARegistry: challenge already resolved");

        challenge.resolved = true;

        for (uint256 i = 0; i < failedValidators.length; i++) {
            address validatorAddr = failedValidators[i];
            Validator storage validator = validators[validatorAddr];

            if (validator.status == ValidatorStatus.Active) {
                uint256 slashAmount = (validator.stake * SLASH_QUANTUM_FAIL_BPS) / 10000;
                validator.stake -= slashAmount;
                validator.slashCount++;
                validator.status = ValidatorStatus.Slashed;

                // Burn slashed tokens
                qbpToken.burn(slashAmount);

                emit ValidatorSlashed(validatorAddr, slashAmount, "Quantum challenge failure");

                // Jail if too many slashes
                if (validator.slashCount >= 3) {
                    validator.status = ValidatorStatus.Jailed;
                    emit ValidatorJailed(validatorAddr, block.number + 50400);
                }
            }
        }
    }

    // ============================================================
    // Block Production Tracking
    // ============================================================

    /**
     * @notice Record that a validator produced a block.
     * @dev Called by the consensus engine for each block.
     * @param validatorAddr Address of the validator.
     * @param blockNumber Block number produced.
     */
    function recordBlockProduction(
        address validatorAddr,
        uint256 blockNumber
    ) external onlyGovernance validatorExists(validatorAddr) {
        validators[validatorAddr].lastBlock = blockNumber;
        validators[validatorAddr].blocksProduced++;
    }

    // ============================================================
    // Validator Exit
    // ============================================================

    /**
     * @notice Request to exit the validator set.
     * @dev Starts the unbonding period. Stake can be withdrawn after the period.
     */
    function requestExit() external onlyActiveValidator {
        validators[msg.sender].status = ValidatorStatus.Exiting;
        pendingWithdrawals[msg.sender] = block.number + unbondingPeriod;
        _deactivateValidator(msg.sender, "Voluntary exit");
    }

    /**
     * @notice Withdraw stake after the unbonding period.
     */
    function withdrawStake() external validatorExists(msg.sender) {
        require(
            validators[msg.sender].status == ValidatorStatus.Exiting,
            "QPoARegistry: not in exiting state"
        );
        require(
            block.number >= pendingWithdrawals[msg.sender],
            "QPoARegistry: unbonding period not complete"
        );

        uint256 amount = validators[msg.sender].stake;
        validators[msg.sender].stake = 0;
        validators[msg.sender].status = ValidatorStatus.Inactive;

        require(
            qbpToken.transfer(msg.sender, amount),
            "QPoARegistry: stake withdrawal failed"
        );

        emit StakeWithdrawn(msg.sender, amount);
    }

    // ============================================================
    // View Functions
    // ============================================================

    /// @notice Get the list of active validators.
    function getActiveValidators() external view returns (address[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < validatorList.length; i++) {
            if (validators[validatorList[i]].status == ValidatorStatus.Active ||
                validators[validatorList[i]].status == ValidatorStatus.Slashed) {
                count++;
            }
        }

        address[] memory active = new address[](count);
        uint256 idx = 0;
        for (uint256 i = 0; i < validatorList.length; i++) {
            if (validators[validatorList[i]].status == ValidatorStatus.Active ||
                validators[validatorList[i]].status == ValidatorStatus.Slashed) {
                active[idx++] = validatorList[i];
            }
        }
        return active;
    }

    /// @notice Get the ML-DSA public key of a validator.
    function getValidatorPublicKey(address validatorAddr) external view returns (bytes memory) {
        return validators[validatorAddr].mldsaPublicKey;
    }

    /// @notice Check if an address is an active validator.
    function isActiveValidator(address addr) external view returns (bool) {
        return validators[addr].status == ValidatorStatus.Active ||
               validators[addr].status == ValidatorStatus.Slashed;
    }

    /// @notice Get validator count.
    function getValidatorCount() external view returns (uint256) {
        return validatorList.length;
    }

    // ============================================================
    // Internal Functions
    // ============================================================

    function _activateValidator(address validatorAddr) internal {
        validators[validatorAddr].status = ValidatorStatus.Active;
        validators[validatorAddr].since = block.number;
        validatorList.push(validatorAddr);
        emit ValidatorActivated(validatorAddr, block.number);
    }

    function _deactivateValidator(address validatorAddr, string memory reason) internal {
        validators[validatorAddr].status = ValidatorStatus.Inactive;

        for (uint256 i = 0; i < validatorList.length; i++) {
            if (validatorList[i] == validatorAddr) {
                validatorList[i] = validatorList[validatorList.length - 1];
                validatorList.pop();
                break;
            }
        }

        emit ValidatorDeactivated(validatorAddr, reason);
    }

    function _verifyMLDSASignature(
        bytes32 messageHash,
        bytes memory publicKey,
        bytes memory signature
    ) internal pure returns (bool) {
        // In production, this calls the QC_VERIFY_PQ opcode
        // For now, basic structural verification
        if (publicKey.length != MLDSA_PUBKEY_SIZE) return false;
        if (signature.length != 3309) return false;

        // Verify signature is not all zeros
        for (uint256 i = 0; i < 32; i++) {
            if (signature[i] != 0) return true;
        }
        return false;
    }
}
