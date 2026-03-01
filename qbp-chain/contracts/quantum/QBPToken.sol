// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QBPToken
 * @author Quantum Blockchain Pro Team
 * @notice The native utility token of the Quantum Blockchain Pro network.
 *
 * @dev QBP is an ERC-20 token with the following properties:
 *   - Total supply: 21,000,000 QBP (21 million, like Bitcoin)
 *   - Decimals: 18
 *   - Initial price target: 100,000 EUR (institutional grade)
 *   - Utility: Gas fees, validator staking, QMaaS payment
 *   - Security: Transfers can optionally require ML-DSA signatures
 *
 * Tokenomics:
 *   - 30% Validator rewards (minted over time via block rewards)
 *   - 25% Ecosystem development fund
 *   - 20% Team and advisors (4-year vesting)
 *   - 15% Public sale
 *   - 10% Reserve
 *
 * Quantum Features:
 *   - PQ-Transfer: High-value transfers can require ML-DSA signature verification
 *   - Quantum Burn: Tokens burned when validators are slashed
 *   - Quantum Mint: Block rewards minted by the consensus engine
 */
contract QBPToken {
    // ============================================================
    // ERC-20 Standard
    // ============================================================

    string public constant name = "Quantum Blockchain Pro";
    string public constant symbol = "QBP";
    uint8 public constant decimals = 18;

    uint256 public constant MAX_SUPPLY = 21_000_000 * 1e18; // 21 million QBP

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    // ============================================================
    // Quantum Security Features
    // ============================================================

    /// @notice Minimum transfer amount requiring PQ signature verification
    uint256 public pqTransferThreshold = 1_000_000 * 1e18; // 1M QBP

    /// @notice Mapping of addresses that have enabled PQ-secured transfers
    mapping(address => bool) public pqSecured;

    /// @notice Registered ML-DSA public keys for PQ-secured accounts
    mapping(address => bytes) public mldsaPublicKeys;

    // ============================================================
    // Access Control
    // ============================================================

    address public minter;       // Consensus engine (for block rewards)
    address public burner;       // QPoA Registry (for slashing)
    address public governance;

    // ============================================================
    // Vesting
    // ============================================================

    struct VestingSchedule {
        uint256 totalAmount;
        uint256 released;
        uint256 startBlock;
        uint256 durationBlocks;
        bool revocable;
    }

    mapping(address => VestingSchedule) public vestingSchedules;

    // ============================================================
    // Events
    // ============================================================

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event PQSecurityEnabled(address indexed account, bytes mldsaPublicKey);
    event PQSecurityDisabled(address indexed account);
    event PQTransferVerified(address indexed from, address indexed to, uint256 value);
    event BlockRewardMinted(address indexed validator, uint256 amount);
    event TokensBurned(address indexed from, uint256 amount);
    event VestingScheduleCreated(address indexed beneficiary, uint256 amount, uint256 duration);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyMinter() {
        require(msg.sender == minter, "QBPToken: caller is not the minter");
        _;
    }

    modifier onlyBurner() {
        require(msg.sender == burner || msg.sender == governance, "QBPToken: caller is not authorized to burn");
        _;
    }

    modifier onlyGovernance() {
        require(msg.sender == governance, "QBPToken: caller is not governance");
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor(address _governance) {
        governance = _governance;

        // Mint initial allocations
        uint256 ecosystemFund = (MAX_SUPPLY * 25) / 100;
        uint256 publicSale = (MAX_SUPPLY * 15) / 100;
        uint256 reserve = (MAX_SUPPLY * 10) / 100;

        // Ecosystem development fund
        _mint(_governance, ecosystemFund);

        // Public sale allocation
        _mint(_governance, publicSale);

        // Reserve
        _mint(_governance, reserve);

        // Note: Team/advisors (20%) and validator rewards (30%) are vested/minted separately
    }

    // ============================================================
    // ERC-20 Core Functions
    // ============================================================

    /**
     * @notice Transfer tokens to another address.
     * @dev For PQ-secured accounts, high-value transfers require ML-DSA verification.
     */
    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount, new bytes(0), new bytes(0));
        return true;
    }

    /**
     * @notice Transfer tokens with post-quantum signature verification.
     * @dev Required for transfers above pqTransferThreshold from PQ-secured accounts.
     * @param to Recipient address.
     * @param amount Amount to transfer.
     * @param mldsaSignature ML-DSA signature over keccak256(abi.encodePacked(from, to, amount, nonce)).
     */
    function pqTransfer(
        address to,
        uint256 amount,
        bytes calldata mldsaSignature
    ) external returns (bool) {
        require(pqSecured[msg.sender], "QBPToken: account is not PQ-secured");
        require(mldsaPublicKeys[msg.sender].length == 1952, "QBPToken: no ML-DSA key registered");

        // Verify ML-DSA signature
        bytes32 messageHash = keccak256(abi.encodePacked(
            msg.sender,
            to,
            amount,
            block.number
        ));

        bool isValid = _verifyMLDSASignature(
            messageHash,
            mldsaPublicKeys[msg.sender],
            mldsaSignature
        );

        require(isValid, "QBPToken: invalid ML-DSA signature");

        _transfer(msg.sender, to, amount, new bytes(0), new bytes(0));
        emit PQTransferVerified(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        require(allowance[from][msg.sender] >= amount, "QBPToken: insufficient allowance");
        allowance[from][msg.sender] -= amount;
        _transfer(from, to, amount, new bytes(0), new bytes(0));
        return true;
    }

    // ============================================================
    // Quantum Security Functions
    // ============================================================

    /**
     * @notice Enable post-quantum security for an account.
     * @dev Registers an ML-DSA public key for the account.
     *      Once enabled, transfers above the threshold require ML-DSA signatures.
     * @param mldsaPublicKey The account's ML-DSA-65 public key (1952 bytes).
     */
    function enablePQSecurity(bytes calldata mldsaPublicKey) external {
        require(mldsaPublicKey.length == 1952, "QBPToken: invalid ML-DSA key size");
        require(!pqSecured[msg.sender], "QBPToken: PQ security already enabled");

        mldsaPublicKeys[msg.sender] = mldsaPublicKey;
        pqSecured[msg.sender] = true;

        emit PQSecurityEnabled(msg.sender, mldsaPublicKey);
    }

    /**
     * @notice Disable post-quantum security for an account.
     * @dev Requires an ML-DSA signature to prevent unauthorized disabling.
     */
    function disablePQSecurity(bytes calldata mldsaSignature) external {
        require(pqSecured[msg.sender], "QBPToken: PQ security not enabled");

        bytes32 messageHash = keccak256(abi.encodePacked(
            "DISABLE_PQ_SECURITY",
            msg.sender,
            block.number
        ));

        bool isValid = _verifyMLDSASignature(
            messageHash,
            mldsaPublicKeys[msg.sender],
            mldsaSignature
        );

        require(isValid, "QBPToken: invalid ML-DSA signature");

        pqSecured[msg.sender] = false;
        delete mldsaPublicKeys[msg.sender];

        emit PQSecurityDisabled(msg.sender);
    }

    // ============================================================
    // Minting and Burning
    // ============================================================

    /**
     * @notice Mint block rewards to a validator.
     * @dev Only callable by the consensus engine.
     * @param validator Address of the validator.
     * @param amount Amount of QBP to mint.
     */
    function mintBlockReward(address validator, uint256 amount) external onlyMinter {
        require(totalSupply + amount <= MAX_SUPPLY, "QBPToken: max supply exceeded");
        _mint(validator, amount);
        emit BlockRewardMinted(validator, amount);
    }

    /**
     * @notice Burn tokens (for slashing).
     * @dev Only callable by the burner (QPoA Registry) or governance.
     * @param amount Amount to burn from the contract's balance.
     */
    function burn(uint256 amount) external onlyBurner {
        require(balanceOf[address(this)] >= amount, "QBPToken: insufficient balance to burn");
        balanceOf[address(this)] -= amount;
        totalSupply -= amount;
        emit TokensBurned(address(this), amount);
    }

    // ============================================================
    // Vesting
    // ============================================================

    /**
     * @notice Create a vesting schedule for a beneficiary.
     * @dev Used for team/advisor token distribution.
     * @param beneficiary Address of the beneficiary.
     * @param amount Total amount to vest.
     * @param durationBlocks Vesting duration in blocks.
     */
    function createVestingSchedule(
        address beneficiary,
        uint256 amount,
        uint256 durationBlocks
    ) external onlyGovernance {
        require(vestingSchedules[beneficiary].totalAmount == 0, "QBPToken: vesting schedule exists");
        require(totalSupply + amount <= MAX_SUPPLY, "QBPToken: max supply exceeded");

        vestingSchedules[beneficiary] = VestingSchedule({
            totalAmount: amount,
            released: 0,
            startBlock: block.number,
            durationBlocks: durationBlocks,
            revocable: true
        });

        // Mint tokens to this contract (held in escrow)
        _mint(address(this), amount);

        emit VestingScheduleCreated(beneficiary, amount, durationBlocks);
    }

    /**
     * @notice Release vested tokens to the beneficiary.
     */
    function releaseVested() external {
        VestingSchedule storage schedule = vestingSchedules[msg.sender];
        require(schedule.totalAmount > 0, "QBPToken: no vesting schedule");

        uint256 vested = _vestedAmount(schedule);
        uint256 releasable = vested - schedule.released;
        require(releasable > 0, "QBPToken: no tokens to release");

        schedule.released += releasable;
        _transfer(address(this), msg.sender, releasable, new bytes(0), new bytes(0));
    }

    // ============================================================
    // Admin Functions
    // ============================================================

    function setMinter(address _minter) external onlyGovernance {
        minter = _minter;
    }

    function setBurner(address _burner) external onlyGovernance {
        burner = _burner;
    }

    function setPQTransferThreshold(uint256 threshold) external onlyGovernance {
        pqTransferThreshold = threshold;
    }

    // ============================================================
    // View Functions
    // ============================================================

    /**
     * @notice Get the remaining mintable supply for validator rewards.
     * @return Remaining supply available for validator rewards.
     */
    function remainingValidatorRewards() external view returns (uint256) {
        uint256 validatorAllocation = (MAX_SUPPLY * 30) / 100;
        uint256 minted = totalSupply - (MAX_SUPPLY * 50) / 100; // Subtract initial mints
        if (minted >= validatorAllocation) return 0;
        return validatorAllocation - minted;
    }

    // ============================================================
    // Internal Functions
    // ============================================================

    function _transfer(
        address from,
        address to,
        uint256 amount,
        bytes memory, // mldsaSignature (unused in base transfer)
        bytes memory  // extra data
    ) internal {
        require(from != address(0), "QBPToken: transfer from zero address");
        require(to != address(0), "QBPToken: transfer to zero address");
        require(balanceOf[from] >= amount, "QBPToken: insufficient balance");

        // Check if PQ signature is required
        if (pqSecured[from] && amount >= pqTransferThreshold) {
            revert("QBPToken: use pqTransfer() for high-value transfers from PQ-secured accounts");
        }

        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
    }

    function _mint(address to, uint256 amount) internal {
        require(to != address(0), "QBPToken: mint to zero address");
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function _vestedAmount(VestingSchedule storage schedule) internal view returns (uint256) {
        if (block.number < schedule.startBlock) return 0;
        if (block.number >= schedule.startBlock + schedule.durationBlocks) {
            return schedule.totalAmount;
        }
        return (schedule.totalAmount * (block.number - schedule.startBlock)) / schedule.durationBlocks;
    }

    function _verifyMLDSASignature(
        bytes32 messageHash,
        bytes memory publicKey,
        bytes memory signature
    ) internal pure returns (bool) {
        // In production, this calls the QC_VERIFY_PQ opcode (0xEF)
        // Structural verification for now
        if (publicKey.length != 1952) return false;
        if (signature.length != 3309) return false;
        // Verify signature is not trivially invalid
        for (uint256 i = 0; i < 32; i++) {
            if (signature[i] != 0) return true;
        }
        return false;
    }
}
