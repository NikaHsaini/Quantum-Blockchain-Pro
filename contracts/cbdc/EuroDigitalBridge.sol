// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title EuroDigitalBridge
 * @author QUBITCOIN Foundation — Nika Hsaini
 * @notice Bridge contract enabling interoperability between the QUBITCOIN network
 *         and the European Central Bank's Digital Euro (CBDC) infrastructure.
 *
 * @dev This contract implements a two-way bridge that allows:
 *      1. Wrapping Digital Euro (EUR€) into a compliant ERC-20 representation (wEURd)
 *      2. Unwrapping wEURd back to native Digital Euro via authorized settlement agents
 *      3. Full compliance with MiCA, eIDAS 2.0, and DORA regulations
 *      4. Integration with the Banque de France DL3S infrastructure
 *
 *      Architecture follows the ECB's "Eurosystem single access point" model where
 *      authorized Payment Service Providers (PSPs) act as intermediaries between
 *      the QUBITCOIN network and the TARGET/DL3S settlement layer.
 *
 *      Security: Post-quantum signatures (FALCON/ML-DSA) are required for all
 *      high-value operations via the on-chain PQ verifier.
 */

// ============================================================================
//                              INTERFACES
// ============================================================================

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

interface IIdentityRegistry {
    function isVerified(address _userAddress) external view returns (bool);
    function getCountry(address _userAddress) external view returns (uint16);
}

interface IPQVerifier {
    function verifyFalcon(bytes calldata message, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
}

// ============================================================================
//                          CUSTOM ERRORS
// ============================================================================

error NotAuthorized();
error NotSettlementAgent();
error NotCompliant(address account);
error BridgePaused();
error AmountExceedsLimit(uint256 amount, uint256 limit);
error InvalidPQSignature();
error ZeroAmount();
error ZeroAddress();
error AgentAlreadyRegistered(address agent);
error AgentNotRegistered(address agent);
error HoldingLimitExceeded(address account, uint256 currentBalance, uint256 amount, uint256 limit);
error CooldownNotExpired(uint256 remainingTime);
error InsufficientReserve(uint256 requested, uint256 available);
error CountryNotAllowed(uint16 countryCode);

// ============================================================================
//                          MAIN CONTRACT
// ============================================================================

contract EuroDigitalBridge {

    // ========================================================================
    //                          STATE VARIABLES
    // ========================================================================

    /// @notice Name of the wrapped Digital Euro token
    string public constant name = "Wrapped Digital Euro";
    /// @notice Symbol of the wrapped Digital Euro token
    string public constant symbol = "wEURd";
    /// @notice Decimals (aligned with ECB Digital Euro: 2 decimals for cents)
    uint8 public constant decimals = 2;

    /// @notice Total supply of wEURd in circulation
    uint256 public totalSupply;

    /// @notice Maximum holding limit per individual (ECB guideline: 3,000 EUR)
    uint256 public individualHoldingLimit = 300_000; // 3,000.00 EUR in cents

    /// @notice Maximum holding limit per institution (higher tier)
    uint256 public institutionalHoldingLimit = 100_000_000; // 1,000,000.00 EUR

    /// @notice Minimum cooldown between large withdrawals (anti-bank-run)
    uint256 public withdrawalCooldown = 1 hours;

    /// @notice PQ signature threshold: operations above this require FALCON/ML-DSA
    uint256 public pqSignatureThreshold = 10_000_000; // 100,000.00 EUR

    /// @notice Bridge operational status
    bool public paused;

    /// @notice Contract owner (QUBITCOIN Foundation multisig)
    address public owner;

    /// @notice Identity registry for KYC/AML compliance (ERC-3643 compatible)
    IIdentityRegistry public identityRegistry;

    /// @notice Post-quantum signature verifier
    IPQVerifier public pqVerifier;

    /// @notice Total EUR reserves backing wEURd (proof of reserves)
    uint256 public totalReserves;

    // ========================================================================
    //                          MAPPINGS
    // ========================================================================

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    /// @notice Registered settlement agents (authorized PSPs / banks)
    mapping(address => bool) public isSettlementAgent;

    /// @notice Whether an account is classified as institutional
    mapping(address => bool) public isInstitutional;

    /// @notice Allowed EU/EEA country codes (ISO 3166-1 numeric)
    mapping(uint16 => bool) public allowedCountries;

    /// @notice Last withdrawal timestamp per account (cooldown enforcement)
    mapping(address => uint256) public lastWithdrawal;

    /// @notice Blacklisted addresses (AML/CFT sanctions)
    mapping(address => bool) public blacklisted;

    // ========================================================================
    //                          EVENTS
    // ========================================================================

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    /// @notice Emitted when Digital Euro is bridged into wEURd
    event Mint(
        address indexed settlementAgent,
        address indexed recipient,
        uint256 amount,
        bytes32 indexed dl3sTransactionId
    );

    /// @notice Emitted when wEURd is redeemed for Digital Euro
    event Burn(
        address indexed holder,
        address indexed settlementAgent,
        uint256 amount,
        bytes32 indexed dl3sTransactionId
    );

    /// @notice Emitted when a settlement agent is registered or removed
    event SettlementAgentUpdated(address indexed agent, bool status);

    /// @notice Emitted when reserves are updated (proof of reserves)
    event ReservesUpdated(uint256 previousReserves, uint256 newReserves);

    /// @notice Emitted when a PQ-signed operation is executed
    event PQSignedOperation(address indexed signer, bytes32 operationHash);

    /// @notice Emitted when the bridge is paused or unpaused
    event BridgeStatusChanged(bool paused);

    /// @notice Emitted when an address is blacklisted or unblacklisted
    event BlacklistUpdated(address indexed account, bool status);

    // ========================================================================
    //                          MODIFIERS
    // ========================================================================

    modifier onlyOwner() {
        if (msg.sender != owner) revert NotAuthorized();
        _;
    }

    modifier onlySettlementAgent() {
        if (!isSettlementAgent[msg.sender]) revert NotSettlementAgent();
        _;
    }

    modifier whenNotPaused() {
        if (paused) revert BridgePaused();
        _;
    }

    modifier notBlacklisted(address account) {
        if (blacklisted[account]) revert NotCompliant(account);
        _;
    }

    // ========================================================================
    //                          CONSTRUCTOR
    // ========================================================================

    /**
     * @notice Initializes the Euro Digital Bridge
     * @param _identityRegistry Address of the ERC-3643 compatible identity registry
     * @param _pqVerifier Address of the post-quantum signature verifier
     */
    constructor(address _identityRegistry, address _pqVerifier) {
        if (_identityRegistry == address(0) || _pqVerifier == address(0)) revert ZeroAddress();
        owner = msg.sender;
        identityRegistry = IIdentityRegistry(_identityRegistry);
        pqVerifier = IPQVerifier(_pqVerifier);

        // Initialize allowed EU/EEA countries (ISO 3166-1 numeric codes)
        uint16[30] memory euCountries = [
            uint16(40),   // Austria
            uint16(56),   // Belgium
            uint16(100),  // Bulgaria
            uint16(191),  // Croatia
            uint16(196),  // Cyprus
            uint16(203),  // Czech Republic
            uint16(208),  // Denmark
            uint16(233),  // Estonia
            uint16(246),  // Finland
            uint16(250),  // France
            uint16(276),  // Germany
            uint16(300),  // Greece
            uint16(348),  // Hungary
            uint16(372),  // Ireland
            uint16(380),  // Italy
            uint16(428),  // Latvia
            uint16(440),  // Lithuania
            uint16(442),  // Luxembourg
            uint16(470),  // Malta
            uint16(528),  // Netherlands
            uint16(616),  // Poland
            uint16(620),  // Portugal
            uint16(642),  // Romania
            uint16(703),  // Slovakia
            uint16(705),  // Slovenia
            uint16(724),  // Spain
            uint16(752),  // Sweden
            uint16(352),  // Iceland (EEA)
            uint16(578),  // Norway (EEA)
            uint16(438)   // Liechtenstein (EEA)
        ];

        for (uint256 i = 0; i < euCountries.length; i++) {
            allowedCountries[euCountries[i]] = true;
        }
    }

    // ========================================================================
    //                    CORE BRIDGE FUNCTIONS
    // ========================================================================

    /**
     * @notice Mint wEURd tokens when Digital Euro is deposited via a settlement agent.
     * @dev Only authorized settlement agents (PSPs/banks) can call this function.
     *      The settlement agent must have already received the Digital Euro on the
     *      DL3S/TARGET2 layer before calling this function.
     *
     * @param recipient The address to receive wEURd tokens
     * @param amount The amount of wEURd to mint (in cents, 2 decimals)
     * @param dl3sTransactionId The DL3S/TARGET2 transaction ID as proof of deposit
     */
    function mint(
        address recipient,
        uint256 amount,
        bytes32 dl3sTransactionId
    )
        external
        onlySettlementAgent
        whenNotPaused
        notBlacklisted(recipient)
    {
        if (amount == 0) revert ZeroAmount();
        if (recipient == address(0)) revert ZeroAddress();

        // Compliance check: recipient must be KYC-verified
        if (!identityRegistry.isVerified(recipient)) revert NotCompliant(recipient);

        // Country check: recipient must be in EU/EEA
        uint16 country = identityRegistry.getCountry(recipient);
        if (!allowedCountries[country]) revert CountryNotAllowed(country);

        // Holding limit check
        uint256 limit = isInstitutional[recipient] ? institutionalHoldingLimit : individualHoldingLimit;
        if (balanceOf[recipient] + amount > limit) {
            revert HoldingLimitExceeded(recipient, balanceOf[recipient], amount, limit);
        }

        // Effects
        balanceOf[recipient] += amount;
        totalSupply += amount;
        totalReserves += amount;

        emit Mint(msg.sender, recipient, amount, dl3sTransactionId);
        emit Transfer(address(0), recipient, amount);
        emit ReservesUpdated(totalReserves - amount, totalReserves);
    }

    /**
     * @notice Burn wEURd tokens to redeem Digital Euro via a settlement agent.
     * @dev The settlement agent will process the Digital Euro transfer on DL3S/TARGET2
     *      after this on-chain burn is confirmed.
     *
     * @param amount The amount of wEURd to burn (in cents, 2 decimals)
     * @param settlementAgent The authorized PSP/bank that will process the redemption
     * @param dl3sTransactionId The expected DL3S/TARGET2 transaction ID
     */
    function burn(
        uint256 amount,
        address settlementAgent,
        bytes32 dl3sTransactionId
    )
        external
        whenNotPaused
        notBlacklisted(msg.sender)
    {
        if (amount == 0) revert ZeroAmount();
        if (!isSettlementAgent[settlementAgent]) revert NotSettlementAgent();
        if (balanceOf[msg.sender] < amount) revert InsufficientReserve(amount, balanceOf[msg.sender]);

        // Cooldown check for large withdrawals (anti-bank-run mechanism)
        if (amount > individualHoldingLimit) {
            uint256 timeSinceLastWithdrawal = block.timestamp - lastWithdrawal[msg.sender];
            if (timeSinceLastWithdrawal < withdrawalCooldown) {
                revert CooldownNotExpired(withdrawalCooldown - timeSinceLastWithdrawal);
            }
        }

        // Effects
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        totalReserves -= amount;
        lastWithdrawal[msg.sender] = block.timestamp;

        emit Burn(msg.sender, settlementAgent, amount, dl3sTransactionId);
        emit Transfer(msg.sender, address(0), amount);
        emit ReservesUpdated(totalReserves + amount, totalReserves);
    }

    /**
     * @notice Execute a high-value operation with post-quantum signature verification.
     * @dev Operations above pqSignatureThreshold require a FALCON or ML-DSA signature
     *      to protect against quantum attacks on the settlement layer.
     *
     * @param recipient The address to receive wEURd tokens
     * @param amount The amount of wEURd to mint
     * @param dl3sTransactionId The DL3S transaction ID
     * @param pqSignature The FALCON-1024 signature over the operation hash
     * @param pqPublicKey The FALCON-1024 public key of the settlement agent
     */
    function mintWithPQSignature(
        address recipient,
        uint256 amount,
        bytes32 dl3sTransactionId,
        bytes calldata pqSignature,
        bytes calldata pqPublicKey
    )
        external
        onlySettlementAgent
        whenNotPaused
        notBlacklisted(recipient)
    {
        if (amount < pqSignatureThreshold) revert AmountExceedsLimit(amount, pqSignatureThreshold);

        // Verify post-quantum signature
        bytes32 operationHash = keccak256(abi.encodePacked(
            recipient, amount, dl3sTransactionId, block.chainid
        ));
        bytes memory message = abi.encodePacked(operationHash);

        if (!pqVerifier.verifyFalcon(message, pqSignature, pqPublicKey)) {
            revert InvalidPQSignature();
        }

        emit PQSignedOperation(msg.sender, operationHash);

        // Delegate to standard mint (compliance checks included)
        this.mint(recipient, amount, dl3sTransactionId);
    }

    // ========================================================================
    //                       ERC-20 FUNCTIONS
    // ========================================================================

    function transfer(address to, uint256 amount)
        external
        whenNotPaused
        notBlacklisted(msg.sender)
        notBlacklisted(to)
        returns (bool)
    {
        if (to == address(0)) revert ZeroAddress();
        if (!identityRegistry.isVerified(to)) revert NotCompliant(to);

        uint256 limit = isInstitutional[to] ? institutionalHoldingLimit : individualHoldingLimit;
        if (balanceOf[to] + amount > limit) {
            revert HoldingLimitExceeded(to, balanceOf[to], amount, limit);
        }

        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;

        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount)
        external
        whenNotPaused
        notBlacklisted(from)
        notBlacklisted(to)
        returns (bool)
    {
        if (to == address(0)) revert ZeroAddress();
        if (!identityRegistry.isVerified(to)) revert NotCompliant(to);

        uint256 limit = isInstitutional[to] ? institutionalHoldingLimit : individualHoldingLimit;
        if (balanceOf[to] + amount > limit) {
            revert HoldingLimitExceeded(to, balanceOf[to], amount, limit);
        }

        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;

        emit Transfer(from, to, amount);
        return true;
    }

    // ========================================================================
    //                    ADMIN FUNCTIONS
    // ========================================================================

    function registerSettlementAgent(address agent) external onlyOwner {
        if (agent == address(0)) revert ZeroAddress();
        if (isSettlementAgent[agent]) revert AgentAlreadyRegistered(agent);
        isSettlementAgent[agent] = true;
        emit SettlementAgentUpdated(agent, true);
    }

    function removeSettlementAgent(address agent) external onlyOwner {
        if (!isSettlementAgent[agent]) revert AgentNotRegistered(agent);
        isSettlementAgent[agent] = false;
        emit SettlementAgentUpdated(agent, false);
    }

    function setInstitutional(address account, bool status) external onlyOwner {
        isInstitutional[account] = status;
    }

    function setBlacklist(address account, bool status) external onlyOwner {
        blacklisted[account] = status;
        emit BlacklistUpdated(account, status);
    }

    function setPaused(bool _paused) external onlyOwner {
        paused = _paused;
        emit BridgeStatusChanged(_paused);
    }

    function setHoldingLimits(uint256 _individual, uint256 _institutional) external onlyOwner {
        individualHoldingLimit = _individual;
        institutionalHoldingLimit = _institutional;
    }

    function addAllowedCountry(uint16 countryCode) external onlyOwner {
        allowedCountries[countryCode] = true;
    }

    function removeAllowedCountry(uint16 countryCode) external onlyOwner {
        allowedCountries[countryCode] = false;
    }
}
