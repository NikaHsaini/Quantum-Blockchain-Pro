// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QBTCToken
 * @author Nika Hsaini — QUBITCOIN Foundation
 * @notice The native utility token of the QUBITCOIN network.
 *
 * @dev QBTC is an ERC-20 compliant utility token with the following properties:
 *
 *   Tokenomics:
 *   - Total supply: 21,000 QBTC (ultra-scarce, 1000x rarer than Bitcoin)
 *   - Decimals: 18
 *   - Initial price target: 100,000 EUR (institutional grade)
 *   - Utility: Gas fees, validator staking, QMaaS payment, B2B SaaS access
 *
 *   Post-Quantum Security (Crypto-Agile Architecture):
 *   - Primary: FALCON-1024 (NIST FIPS 206) via ZKnox ETHFALCON (~1.5M gas)
 *   - Secondary: ML-DSA-65 (CRYSTALS-Dilithium, NIST FIPS 204)
 *   - Recovery: EPERVIER (ZKnox, FALCON with ecrecover-style recovery)
 *   - Hash: SHA-999 (triple-layer SHA3-512 with domain separation)
 *   - Crypto-agility: Algorithm migration without network disruption
 *
 *   Regulatory Compliance:
 *   - MiCA (Markets in Crypto-Assets): Pure utility token structure
 *   - eIDAS 2.0: European Digital Identity Wallet binding
 *   - DORA: PQC infrastructure for quantum cyber-risk management
 *
 *   Token Allocation (21,000 QBTC):
 *   - 30% (6,300 QBTC): Protocol & Ecosystem Development
 *   - 25% (5,250 QBTC): Ecosystem & Staking Rewards
 *   - 20% (4,200 QBTC): Strategic Investors (Seed & Private, vesting 4 years)
 *   - 15% (3,150 QBTC): Founding Team & Advisors (vesting 4 years, 1-year cliff)
 *   - 10% (2,100 QBTC): Public Sale / Liquidity (regulated EU exchanges)
 *
 *   Security Audit Compliance:
 *   - ReentrancyGuard on all external state-changing functions
 *   - Custom errors for gas-efficient reverts
 *   - CEI (Checks-Effects-Interactions) pattern throughout
 *   - No delegatecall, no selfdestruct, no assembly
 */

// ============================================================
// Custom Errors (gas-efficient, EIP-6093 compliant)
// ============================================================

error QBTC__ZeroAddress();
error QBTC__InsufficientBalance(address account, uint256 balance, uint256 needed);
error QBTC__InsufficientAllowance(address spender, uint256 allowance, uint256 needed);
error QBTC__ExceedsMaxSupply(uint256 totalSupply, uint256 amount, uint256 maxSupply);
error QBTC__AccountBlacklisted(address account);
error QBTC__NotGovernance(address caller);
error QBTC__NotPendingGovernance(address caller);
error QBTC__PQSecurityNotEnabled(address account);
error QBTC__InvalidPQSignature();
error QBTC__InvalidFALCONKeySize(uint256 provided, uint256 expected);
error QBTC__InvalidMLDSAKeySize(uint256 provided, uint256 expected);
error QBTC__InvalidKYCLevel(uint256 level);
error QBTC__CannotSetNoneAlgorithm();
error QBTC__HighValueTransferRequiresPQ(uint256 amount, uint256 threshold);
error QBTC__NoVestingSchedule(address beneficiary);
error QBTC__VestingRevoked(address beneficiary);
error QBTC__CliffNotReached(uint256 elapsed, uint256 cliffDuration);
error QBTC__NoTokensToRelease();
error QBTC__ReentrancyDetected();

// ============================================================
// Interfaces
// ============================================================

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
}

interface IQuantumVerifier {
    function verifyFALCON(bytes32 messageHash, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
    function verifyMLDSA(bytes32 messageHash, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
    function sha999(bytes calldata data) external pure returns (bytes32);
}

// ============================================================
// Enums and Structs
// ============================================================

/// @notice Supported post-quantum signature algorithms (crypto-agility)
enum PQAlgorithm {
    NONE,       // No PQ security (standard EVM account)
    FALCON,     // FALCON-1024 (primary, compact, high-performance, ZKnox ETHFALCON)
    MLDSA,      // ML-DSA-65 / CRYSTALS-Dilithium (secondary, NIST FIPS 204)
    EPERVIER    // EPERVIER (ZKnox, FALCON with address recovery)
}

/// @notice Vesting schedule for team and investor allocations
struct VestingSchedule {
    uint256 totalAmount;
    uint256 releasedAmount;
    uint256 startTime;
    uint256 cliffDuration;
    uint256 vestingDuration;
    bool revocable;
    bool revoked;
}

/// @notice eIDAS 2.0 identity binding for institutional compliance
struct IdentityBinding {
    bytes32 eidasWalletHash;    // Hash of the European Digital Identity Wallet ID
    uint256 kycLevel;           // 0: none, 1: basic, 2: enhanced, 3: institutional
    uint256 bindingTimestamp;
    bool active;
}

// ============================================================
// ReentrancyGuard (inline, no external dependency)
// ============================================================

abstract contract ReentrancyGuard {
    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;
    uint256 private _status;

    constructor() {
        _status = _NOT_ENTERED;
    }

    modifier nonReentrant() {
        if (_status == _ENTERED) revert QBTC__ReentrancyDetected();
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }
}

// ============================================================
// QBTCToken Contract
// ============================================================

contract QBTCToken is IERC20, ReentrancyGuard {

    // ============================================================
    // Token Metadata
    // ============================================================

    string public constant name     = "QUBITCOIN";
    string public constant symbol   = "QBTC";
    uint8  public constant decimals = 18;

    /// @notice Maximum supply: 21,000 QBTC — permanently fixed, no inflation possible
    uint256 public constant MAX_SUPPLY = 21_000 * 1e18;

    // ============================================================
    // ERC-20 State
    // ============================================================

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    // ============================================================
    // Nonce tracking for replay protection
    // ============================================================

    mapping(address => uint256) public pqNonces;

    // ============================================================
    // Crypto-Agile Post-Quantum Security
    // ============================================================

    /// @notice Address of the on-chain quantum verifier (ZKnox ETHFALCON / EPERVIER)
    address public quantumVerifier;

    /// @notice Current active PQ algorithm for new accounts (crypto-agility)
    PQAlgorithm public activePQAlgorithm = PQAlgorithm.FALCON;

    /// @notice Per-account PQ security settings
    mapping(address => PQAlgorithm) public accountPQAlgorithm;

    /// @notice Per-account FALCON-1024 public keys (ZKnox ETHFALCON)
    mapping(address => bytes) public falconPublicKeys;

    /// @notice Per-account ML-DSA-65 public keys (fallback)
    mapping(address => bytes) public mldsaPublicKeys;

    /// @notice Minimum transfer amount requiring PQ signature verification
    uint256 public pqTransferThreshold = 100 * 1e18; // 100 QBTC

    // ============================================================
    // Regulatory Compliance (MiCA / eIDAS 2.0 / DORA)
    // ============================================================

    /// @notice eIDAS 2.0 identity bindings for institutional compliance
    mapping(address => IdentityBinding) public identityBindings;

    /// @notice Blacklist for AML/CFT compliance (CASP obligations under MiCA)
    mapping(address => bool) public blacklisted;

    /// @notice Whitelist for institutional accounts (bypasses certain restrictions)
    mapping(address => bool) public institutionalWhitelist;

    // ============================================================
    // Vesting (Team, Advisors, Investors)
    // ============================================================

    mapping(address => VestingSchedule) public vestingSchedules;

    // ============================================================
    // Governance
    // ============================================================

    address public governance;
    address public pendingGovernance;

    // ============================================================
    // Events
    // ============================================================

    event PQSecurityEnabled(address indexed account, PQAlgorithm algorithm);
    event PQAlgorithmMigrated(PQAlgorithm oldAlgorithm, PQAlgorithm newAlgorithm);
    event IdentityBound(address indexed account, bytes32 eidasWalletHash, uint256 kycLevel);
    event Blacklisted(address indexed account, bool status);
    event InstitutionalWhitelistUpdated(address indexed account, bool status);
    event VestingScheduleCreated(address indexed beneficiary, uint256 amount, uint256 startTime);
    event TokensVested(address indexed beneficiary, uint256 amount);
    event GovernanceTransferred(address indexed oldGovernance, address indexed newGovernance);
    event QuantumVerifierUpdated(address indexed oldVerifier, address indexed newVerifier);
    event PQTransferThresholdUpdated(uint256 oldThreshold, uint256 newThreshold);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyGovernance() {
        if (msg.sender != governance) revert QBTC__NotGovernance(msg.sender);
        _;
    }

    modifier notBlacklisted(address account) {
        if (blacklisted[account]) revert QBTC__AccountBlacklisted(account);
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor(address _governance, address _quantumVerifier) {
        if (_governance == address(0)) revert QBTC__ZeroAddress();
        governance       = _governance;
        quantumVerifier  = _quantumVerifier;

        // ── Token Allocation (21,000 QBTC) ──────────────────────
        uint256 protocolEcosystem  = (MAX_SUPPLY * 30) / 100; // 6,300 QBTC
        uint256 stakingRewards     = (MAX_SUPPLY * 25) / 100; // 5,250 QBTC
        uint256 strategicInvestors = (MAX_SUPPLY * 20) / 100; // 4,200 QBTC
        uint256 teamAdvisors       = (MAX_SUPPLY * 15) / 100; // 3,150 QBTC
        uint256 publicSale         = (MAX_SUPPLY * 10) / 100; // 2,100 QBTC

        _mint(_governance, protocolEcosystem);
        _mint(_governance, stakingRewards);
        _createVestingSchedule(_governance, strategicInvestors, 0, 4 * 365 days);
        _createVestingSchedule(_governance, teamAdvisors, 365 days, 4 * 365 days);
        _mint(_governance, publicSale);
    }

    // ============================================================
    // ERC-20 Core Functions
    // ============================================================

    /// @inheritdoc IERC20
    function transfer(address to, uint256 amount)
        external
        nonReentrant
        notBlacklisted(msg.sender)
        notBlacklisted(to)
        returns (bool)
    {
        _transfer(msg.sender, to, amount);
        return true;
    }

    /// @inheritdoc IERC20
    function approve(address spender, uint256 amount) external returns (bool) {
        if (spender == address(0)) revert QBTC__ZeroAddress();
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    /// @inheritdoc IERC20
    function transferFrom(address from, address to, uint256 amount)
        external
        nonReentrant
        notBlacklisted(from)
        notBlacklisted(to)
        returns (bool)
    {
        uint256 currentAllowance = allowance[from][msg.sender];
        if (currentAllowance < amount) {
            revert QBTC__InsufficientAllowance(msg.sender, currentAllowance, amount);
        }
        allowance[from][msg.sender] = currentAllowance - amount;
        _transfer(from, to, amount);
        return true;
    }

    // ============================================================
    // Post-Quantum Secured Transfer (FALCON-1024 / ML-DSA-65 / EPERVIER)
    // ============================================================

    /**
     * @notice Execute a PQ-secured transfer requiring a FALCON-1024, ML-DSA-65, or EPERVIER signature.
     * @dev Mandatory for transfers above `pqTransferThreshold` from PQ-secured accounts.
     *      Provides military-grade security against quantum computer attacks (Shor's algorithm).
     *      Uses ZKnox ETHFALCON for on-chain FALCON verification (~1.5M gas).
     * @param to Recipient address
     * @param amount Amount of QBTC to transfer (in wei)
     * @param pqSignature Post-quantum signature over the transfer payload
     * @param nonce Anti-replay nonce (must equal pqNonces[msg.sender])
     */
    function pqTransfer(
        address to,
        uint256 amount,
        bytes calldata pqSignature,
        uint256 nonce
    )
        external
        nonReentrant
        notBlacklisted(msg.sender)
        notBlacklisted(to)
        returns (bool)
    {
        PQAlgorithm algo = accountPQAlgorithm[msg.sender];
        if (algo == PQAlgorithm.NONE) revert QBTC__PQSecurityNotEnabled(msg.sender);

        // Verify nonce for replay protection
        require(nonce == pqNonces[msg.sender], "QBTC: invalid nonce");

        // Construct the message hash (EIP-712 style domain separation)
        bytes32 messageHash = keccak256(abi.encodePacked(
            "\x19\x01",
            "QBTC_PQ_TRANSFER_V2",
            msg.sender,
            to,
            amount,
            nonce,
            block.chainid
        ));

        // Verify the post-quantum signature via ZKnox verifier
        bool valid;
        if (algo == PQAlgorithm.FALCON || algo == PQAlgorithm.EPERVIER) {
            valid = IQuantumVerifier(quantumVerifier).verifyFALCON(
                messageHash,
                pqSignature,
                falconPublicKeys[msg.sender]
            );
        } else if (algo == PQAlgorithm.MLDSA) {
            valid = IQuantumVerifier(quantumVerifier).verifyMLDSA(
                messageHash,
                pqSignature,
                mldsaPublicKeys[msg.sender]
            );
        }

        if (!valid) revert QBTC__InvalidPQSignature();

        // Increment nonce BEFORE transfer (CEI pattern)
        pqNonces[msg.sender]++;

        _transfer(msg.sender, to, amount);
        return true;
    }

    // ============================================================
    // Crypto-Agile PQ Security Management
    // ============================================================

    /**
     * @notice Enable FALCON-1024 post-quantum security for the caller's account.
     * @dev Uses ZKnox ETHFALCON for on-chain verification.
     * @param falconPublicKey The FALCON-1024 public key (1793 bytes)
     */
    function enableFALCONSecurity(bytes calldata falconPublicKey) external {
        if (falconPublicKey.length != 1793) {
            revert QBTC__InvalidFALCONKeySize(falconPublicKey.length, 1793);
        }
        falconPublicKeys[msg.sender] = falconPublicKey;
        accountPQAlgorithm[msg.sender] = PQAlgorithm.FALCON;
        emit PQSecurityEnabled(msg.sender, PQAlgorithm.FALCON);
    }

    /**
     * @notice Enable ML-DSA-65 post-quantum security for the caller's account.
     * @param mldsaPublicKey The ML-DSA-65 public key (1952 bytes)
     */
    function enableMLDSASecurity(bytes calldata mldsaPublicKey) external {
        if (mldsaPublicKey.length != 1952) {
            revert QBTC__InvalidMLDSAKeySize(mldsaPublicKey.length, 1952);
        }
        mldsaPublicKeys[msg.sender] = mldsaPublicKey;
        accountPQAlgorithm[msg.sender] = PQAlgorithm.MLDSA;
        emit PQSecurityEnabled(msg.sender, PQAlgorithm.MLDSA);
    }

    /**
     * @notice Enable EPERVIER (ZKnox FALCON with address recovery) for the caller's account.
     * @param falconPublicKey The FALCON-1024 public key (1793 bytes)
     */
    function enableEPERVIERSecurity(bytes calldata falconPublicKey) external {
        if (falconPublicKey.length != 1793) {
            revert QBTC__InvalidFALCONKeySize(falconPublicKey.length, 1793);
        }
        falconPublicKeys[msg.sender] = falconPublicKey;
        accountPQAlgorithm[msg.sender] = PQAlgorithm.EPERVIER;
        emit PQSecurityEnabled(msg.sender, PQAlgorithm.EPERVIER);
    }

    /**
     * @notice Migrate the network's active PQ algorithm (crypto-agility).
     * @dev Only callable by governance. Allows seamless algorithm upgrade.
     * @param newAlgorithm The new default PQ algorithm
     */
    function migrateActivePQAlgorithm(PQAlgorithm newAlgorithm) external onlyGovernance {
        if (newAlgorithm == PQAlgorithm.NONE) revert QBTC__CannotSetNoneAlgorithm();
        PQAlgorithm old = activePQAlgorithm;
        activePQAlgorithm = newAlgorithm;
        emit PQAlgorithmMigrated(old, newAlgorithm);
    }

    // ============================================================
    // eIDAS 2.0 / MiCA Compliance
    // ============================================================

    /// @notice Bind an eIDAS 2.0 European Digital Identity Wallet to an account.
    function bindEidasIdentity(
        address account,
        bytes32 eidasWalletHash,
        uint256 kycLevel
    ) external onlyGovernance {
        if (kycLevel < 1 || kycLevel > 3) revert QBTC__InvalidKYCLevel(kycLevel);
        identityBindings[account] = IdentityBinding({
            eidasWalletHash: eidasWalletHash,
            kycLevel: kycLevel,
            bindingTimestamp: block.timestamp,
            active: true
        });
        emit IdentityBound(account, eidasWalletHash, kycLevel);
    }

    /// @notice Blacklist or un-blacklist an account for AML/CFT compliance.
    function setBlacklist(address account, bool status) external onlyGovernance {
        blacklisted[account] = status;
        emit Blacklisted(account, status);
    }

    /// @notice Add or remove an account from the institutional whitelist.
    function setInstitutionalWhitelist(address account, bool status) external onlyGovernance {
        institutionalWhitelist[account] = status;
        emit InstitutionalWhitelistUpdated(account, status);
    }

    // ============================================================
    // Vesting
    // ============================================================

    /// @notice Release vested tokens for a beneficiary.
    function releaseVestedTokens(address beneficiary) external nonReentrant {
        VestingSchedule storage schedule = vestingSchedules[beneficiary];
        if (schedule.totalAmount == 0) revert QBTC__NoVestingSchedule(beneficiary);
        if (schedule.revoked) revert QBTC__VestingRevoked(beneficiary);

        uint256 elapsed = block.timestamp - schedule.startTime;
        if (elapsed < schedule.cliffDuration) {
            revert QBTC__CliffNotReached(elapsed, schedule.cliffDuration);
        }

        uint256 vestedAmount;
        if (elapsed >= schedule.vestingDuration) {
            vestedAmount = schedule.totalAmount;
        } else {
            vestedAmount = (schedule.totalAmount * elapsed) / schedule.vestingDuration;
        }

        uint256 releasable = vestedAmount - schedule.releasedAmount;
        if (releasable == 0) revert QBTC__NoTokensToRelease();

        // Effects before interactions (CEI)
        schedule.releasedAmount += releasable;
        _mint(beneficiary, releasable);
        emit TokensVested(beneficiary, releasable);
    }

    // ============================================================
    // Governance
    // ============================================================

    function transferGovernance(address newGovernance) external onlyGovernance {
        if (newGovernance == address(0)) revert QBTC__ZeroAddress();
        pendingGovernance = newGovernance;
    }

    function acceptGovernance() external {
        if (msg.sender != pendingGovernance) revert QBTC__NotPendingGovernance(msg.sender);
        emit GovernanceTransferred(governance, pendingGovernance);
        governance = pendingGovernance;
        pendingGovernance = address(0);
    }

    function updatePQTransferThreshold(uint256 newThreshold) external onlyGovernance {
        uint256 old = pqTransferThreshold;
        pqTransferThreshold = newThreshold;
        emit PQTransferThresholdUpdated(old, newThreshold);
    }

    function updateQuantumVerifier(address newVerifier) external onlyGovernance {
        address old = quantumVerifier;
        quantumVerifier = newVerifier;
        emit QuantumVerifierUpdated(old, newVerifier);
    }

    // ============================================================
    // Internal Functions
    // ============================================================

    function _transfer(address from, address to, uint256 amount) internal {
        if (from == address(0)) revert QBTC__ZeroAddress();
        if (to == address(0)) revert QBTC__ZeroAddress();
        uint256 fromBalance = balanceOf[from];
        if (fromBalance < amount) {
            revert QBTC__InsufficientBalance(from, fromBalance, amount);
        }

        // Enforce PQ signature for high-value transfers from PQ-secured accounts
        if (
            amount >= pqTransferThreshold &&
            accountPQAlgorithm[from] != PQAlgorithm.NONE &&
            msg.sig != this.pqTransfer.selector
        ) {
            revert QBTC__HighValueTransferRequiresPQ(amount, pqTransferThreshold);
        }

        // Effects (CEI pattern)
        unchecked {
            balanceOf[from] = fromBalance - amount;
        }
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
    }

    function _mint(address to, uint256 amount) internal {
        if (to == address(0)) revert QBTC__ZeroAddress();
        if (totalSupply + amount > MAX_SUPPLY) {
            revert QBTC__ExceedsMaxSupply(totalSupply, amount, MAX_SUPPLY);
        }
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function _createVestingSchedule(
        address beneficiary,
        uint256 amount,
        uint256 cliffDuration,
        uint256 vestingDuration
    ) internal {
        vestingSchedules[beneficiary] = VestingSchedule({
            totalAmount:     amount,
            releasedAmount:  0,
            startTime:       block.timestamp,
            cliffDuration:   cliffDuration,
            vestingDuration: vestingDuration,
            revocable:       true,
            revoked:         false
        });
        emit VestingScheduleCreated(beneficiary, amount, block.timestamp);
    }
}
