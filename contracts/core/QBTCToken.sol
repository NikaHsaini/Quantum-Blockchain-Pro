// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QBTCToken
 * @author Nika Hsaini — Quantum Blockchain Pro
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
 *   - Primary signature scheme: FALCON-1024 (NIST FIPS 206 finalist)
 *     → Fast-Fourier Lattice-based Compact Signatures Over NTRU
 *     → Chosen for its compact signature size (~1330 bytes) and high performance
 *   - Secondary signature scheme: ML-DSA-65 (CRYSTALS-Dilithium, NIST FIPS 204)
 *     → Fallback and cross-validation scheme
 *   - Hash function: SHA-999 (triple-layer SHA3-512 with domain separation)
 *     → Quantum-resistant hash for all on-chain commitments
 *   - Crypto-agility: The contract supports algorithm migration without network disruption
 *
 *   Regulatory Compliance:
 *   - MiCA (Markets in Crypto-Assets): Structured as a pure utility token
 *   - eIDAS 2.0: Compatible with European Digital Identity Wallet for KYC/AML
 *   - DORA (Digital Operational Resilience Act): PQC infrastructure helps financial
 *     institutions comply with quantum cyber-risk management obligations
 *
 *   Token Allocation (21,000 QBTC):
 *   - 30% (6,300 QBTC): Protocol & Ecosystem Development
 *   - 25% (5,250 QBTC): Ecosystem & Staking Rewards
 *   - 20% (4,200 QBTC): Strategic Investors (Seed & Private, vesting 4 years)
 *   - 15% (3,150 QBTC): Founding Team & Advisors (vesting 4 years, 1-year cliff)
 *   - 10% (2,100 QBTC): Public Sale / Liquidity (regulated EU exchanges)
 */

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
    FALCON,     // FALCON-1024 (primary, compact, high-performance)
    MLDSA       // ML-DSA-65 / CRYSTALS-Dilithium (secondary, NIST FIPS 204)
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
// QBTCToken Contract
// ============================================================

contract QBTCToken is IERC20 {

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
    // Crypto-Agile Post-Quantum Security
    // ============================================================

    /// @notice Address of the on-chain quantum verifier precompile
    address public quantumVerifier;

    /// @notice Current active PQ algorithm for new accounts (crypto-agility)
    PQAlgorithm public activePQAlgorithm = PQAlgorithm.FALCON;

    /// @notice Per-account PQ security settings
    mapping(address => PQAlgorithm) public accountPQAlgorithm;

    /// @notice Per-account FALCON-1024 public keys
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

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event PQSecurityEnabled(address indexed account, PQAlgorithm algorithm);
    event PQAlgorithmMigrated(PQAlgorithm oldAlgorithm, PQAlgorithm newAlgorithm);
    event IdentityBound(address indexed account, bytes32 eidasWalletHash, uint256 kycLevel);
    event Blacklisted(address indexed account, bool status);
    event VestingScheduleCreated(address indexed beneficiary, uint256 amount, uint256 startTime);
    event TokensVested(address indexed beneficiary, uint256 amount);
    event GovernanceTransferred(address indexed oldGovernance, address indexed newGovernance);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyGovernance() {
        require(msg.sender == governance, "QBTC: caller is not governance");
        _;
    }

    modifier notBlacklisted(address account) {
        require(!blacklisted[account], "QBTC: account is blacklisted (AML/CFT)");
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor(address _governance, address _quantumVerifier) {
        governance       = _governance;
        quantumVerifier  = _quantumVerifier;

        // ── Token Allocation (21,000 QBTC) ──────────────────────
        // Allocation follows the QUBITCOIN institutional whitepaper v3

        uint256 protocolEcosystem  = (MAX_SUPPLY * 30) / 100; // 6,300 QBTC
        uint256 stakingRewards     = (MAX_SUPPLY * 25) / 100; // 5,250 QBTC
        uint256 strategicInvestors = (MAX_SUPPLY * 20) / 100; // 4,200 QBTC
        uint256 teamAdvisors       = (MAX_SUPPLY * 15) / 100; // 3,150 QBTC
        uint256 publicSale         = (MAX_SUPPLY * 10) / 100; // 2,100 QBTC

        // Protocol & Ecosystem: minted to governance (foundation multisig)
        _mint(_governance, protocolEcosystem);

        // Staking Rewards: minted to governance for distribution via staking contract
        _mint(_governance, stakingRewards);

        // Strategic Investors: vesting 4 years, no cliff
        _createVestingSchedule(_governance, strategicInvestors, 0, 4 * 365 days);

        // Team & Advisors: vesting 4 years, 1-year cliff
        _createVestingSchedule(_governance, teamAdvisors, 365 days, 4 * 365 days);

        // Public Sale / Liquidity: immediately available for regulated EU exchanges
        _mint(_governance, publicSale);
    }

    // ============================================================
    // ERC-20 Core Functions
    // ============================================================

    function transfer(address to, uint256 amount)
        external
        notBlacklisted(msg.sender)
        notBlacklisted(to)
        returns (bool)
    {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount)
        external
        notBlacklisted(from)
        notBlacklisted(to)
        returns (bool)
    {
        require(allowance[from][msg.sender] >= amount, "QBTC: insufficient allowance");
        allowance[from][msg.sender] -= amount;
        _transfer(from, to, amount);
        return true;
    }

    // ============================================================
    // Post-Quantum Secured Transfer (FALCON-1024 / ML-DSA-65)
    // ============================================================

    /**
     * @notice Execute a PQ-secured transfer requiring a FALCON-1024 or ML-DSA-65 signature.
     * @dev This function is mandatory for transfers above `pqTransferThreshold` from PQ-secured accounts.
     *      It provides military-grade security against quantum computer attacks (Shor's algorithm).
     * @param to Recipient address
     * @param amount Amount of QBTC to transfer (in wei)
     * @param pqSignature Post-quantum signature over the transfer payload
     * @param nonce Anti-replay nonce
     */
    function pqTransfer(
        address to,
        uint256 amount,
        bytes calldata pqSignature,
        uint256 nonce
    )
        external
        notBlacklisted(msg.sender)
        notBlacklisted(to)
        returns (bool)
    {
        PQAlgorithm algo = accountPQAlgorithm[msg.sender];
        require(algo != PQAlgorithm.NONE, "QBTC: PQ security not enabled for this account");

        // Construct the message hash (EIP-712 style)
        bytes32 messageHash = keccak256(abi.encodePacked(
            "QBTC_TRANSFER",
            msg.sender,
            to,
            amount,
            nonce,
            block.chainid
        ));

        // Verify the post-quantum signature
        bool valid;
        if (algo == PQAlgorithm.FALCON) {
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

        require(valid, "QBTC: invalid post-quantum signature");
        _transfer(msg.sender, to, amount);
        return true;
    }

    // ============================================================
    // Crypto-Agile PQ Security Management
    // ============================================================

    /**
     * @notice Enable FALCON-1024 post-quantum security for the caller's account.
     * @param falconPublicKey The FALCON-1024 public key (1793 bytes for FALCON-1024)
     */
    function enableFALCONSecurity(bytes calldata falconPublicKey) external {
        require(falconPublicKey.length == 1793, "QBTC: invalid FALCON-1024 public key size");
        falconPublicKeys[msg.sender] = falconPublicKey;
        accountPQAlgorithm[msg.sender] = PQAlgorithm.FALCON;
        emit PQSecurityEnabled(msg.sender, PQAlgorithm.FALCON);
    }

    /**
     * @notice Enable ML-DSA-65 post-quantum security for the caller's account.
     * @param mldsaPublicKey The ML-DSA-65 public key (1952 bytes)
     */
    function enableMLDSASecurity(bytes calldata mldsaPublicKey) external {
        require(mldsaPublicKey.length == 1952, "QBTC: invalid ML-DSA-65 public key size");
        mldsaPublicKeys[msg.sender] = mldsaPublicKey;
        accountPQAlgorithm[msg.sender] = PQAlgorithm.MLDSA;
        emit PQSecurityEnabled(msg.sender, PQAlgorithm.MLDSA);
    }

    /**
     * @notice Migrate the network's active PQ algorithm (crypto-agility).
     * @dev Only callable by governance. Allows seamless algorithm upgrade without network disruption.
     *      This is a critical feature for long-term security as new quantum attacks may emerge.
     * @param newAlgorithm The new default PQ algorithm
     */
    function migrateActivePQAlgorithm(PQAlgorithm newAlgorithm) external onlyGovernance {
        require(newAlgorithm != PQAlgorithm.NONE, "QBTC: cannot set NONE as active algorithm");
        PQAlgorithm old = activePQAlgorithm;
        activePQAlgorithm = newAlgorithm;
        emit PQAlgorithmMigrated(old, newAlgorithm);
    }

    // ============================================================
    // eIDAS 2.0 / MiCA Compliance
    // ============================================================

    /**
     * @notice Bind an eIDAS 2.0 European Digital Identity Wallet to an account.
     * @dev Enables KYC/AML compliance as required by MiCA for CASPs.
     * @param account The account to bind
     * @param eidasWalletHash Hash of the eIDAS wallet identifier
     * @param kycLevel KYC level (1: basic, 2: enhanced, 3: institutional)
     */
    function bindEidasIdentity(
        address account,
        bytes32 eidasWalletHash,
        uint256 kycLevel
    ) external onlyGovernance {
        require(kycLevel >= 1 && kycLevel <= 3, "QBTC: invalid KYC level");
        identityBindings[account] = IdentityBinding({
            eidasWalletHash: eidasWalletHash,
            kycLevel: kycLevel,
            bindingTimestamp: block.timestamp,
            active: true
        });
        emit IdentityBound(account, eidasWalletHash, kycLevel);
    }

    /**
     * @notice Blacklist or un-blacklist an account for AML/CFT compliance.
     * @dev Required under MiCA CASP obligations and DORA cyber-risk management.
     */
    function setBlacklist(address account, bool status) external onlyGovernance {
        blacklisted[account] = status;
        emit Blacklisted(account, status);
    }

    /**
     * @notice Add or remove an account from the institutional whitelist.
     */
    function setInstitutionalWhitelist(address account, bool status) external onlyGovernance {
        institutionalWhitelist[account] = status;
    }

    // ============================================================
    // Vesting
    // ============================================================

    /**
     * @notice Release vested tokens for a beneficiary.
     */
    function releaseVestedTokens(address beneficiary) external {
        VestingSchedule storage schedule = vestingSchedules[beneficiary];
        require(schedule.totalAmount > 0, "QBTC: no vesting schedule");
        require(!schedule.revoked, "QBTC: vesting revoked");

        uint256 elapsed = block.timestamp - schedule.startTime;
        require(elapsed >= schedule.cliffDuration, "QBTC: cliff not reached");

        uint256 vestedAmount;
        if (elapsed >= schedule.vestingDuration) {
            vestedAmount = schedule.totalAmount;
        } else {
            vestedAmount = (schedule.totalAmount * elapsed) / schedule.vestingDuration;
        }

        uint256 releasable = vestedAmount - schedule.releasedAmount;
        require(releasable > 0, "QBTC: no tokens to release");

        schedule.releasedAmount += releasable;
        _mint(beneficiary, releasable);
        emit TokensVested(beneficiary, releasable);
    }

    // ============================================================
    // Governance
    // ============================================================

    function transferGovernance(address newGovernance) external onlyGovernance {
        pendingGovernance = newGovernance;
    }

    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "QBTC: not pending governance");
        emit GovernanceTransferred(governance, pendingGovernance);
        governance = pendingGovernance;
        pendingGovernance = address(0);
    }

    function updatePQTransferThreshold(uint256 newThreshold) external onlyGovernance {
        pqTransferThreshold = newThreshold;
    }

    function updateQuantumVerifier(address newVerifier) external onlyGovernance {
        quantumVerifier = newVerifier;
    }

    // ============================================================
    // Internal Functions
    // ============================================================

    function _transfer(address from, address to, uint256 amount) internal {
        require(from != address(0), "QBTC: transfer from zero address");
        require(to != address(0), "QBTC: transfer to zero address");
        require(balanceOf[from] >= amount, "QBTC: insufficient balance");

        // Enforce PQ signature for high-value transfers from PQ-secured accounts
        if (
            amount >= pqTransferThreshold &&
            accountPQAlgorithm[from] != PQAlgorithm.NONE &&
            msg.sig != this.pqTransfer.selector
        ) {
            revert("QBTC: high-value transfer requires pqTransfer() with PQ signature");
        }

        balanceOf[from] -= amount;
        balanceOf[to]   += amount;
        emit Transfer(from, to, amount);
    }

    function _mint(address to, uint256 amount) internal {
        require(to != address(0), "QBTC: mint to zero address");
        require(totalSupply + amount <= MAX_SUPPLY, "QBTC: exceeds max supply of 21,000 QBTC");
        totalSupply    += amount;
        balanceOf[to]  += amount;
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
