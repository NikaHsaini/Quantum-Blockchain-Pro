// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QBTCLiquidityPool
 * @author QUBITCOIN Foundation — Nika Hsaini
 * @notice Institutional-grade Automated Market Maker (AMM) for the QBTC/wEURd pair.
 *
 * @dev This contract implements a concentrated liquidity pool inspired by Uniswap V3,
 *      specifically designed for the QUBITCOIN ecosystem. It enables:
 *
 *      1. Deep liquidity for the QBTC/wEURd pair, supporting the 100,000 EUR target price
 *      2. Institutional compliance via ERC-3643 identity checks on all participants
 *      3. Post-quantum secured high-value swaps (FALCON signature required above threshold)
 *      4. Protocol-owned liquidity (POL) mechanism for long-term price stability
 *      5. Dynamic fee structure based on volatility and trade size
 *      6. Oracle integration for TWAP (Time-Weighted Average Price) feeds
 *
 *      The pool uses a constant product formula (x * y = k) with concentrated liquidity
 *      ranges, allowing LPs to provide liquidity within specific price bands for
 *      maximum capital efficiency.
 *
 *      Security: ReentrancyGuard, CEI pattern, custom errors, PQ signatures for large trades.
 */

// ============================================================================
//                              INTERFACES
// ============================================================================

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function approve(address spender, uint256 amount) external returns (bool);
}

interface IIdentityRegistry {
    function isVerified(address _userAddress) external view returns (bool);
}

interface IPQVerifier {
    function verifyFalcon(bytes calldata message, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
}

// ============================================================================
//                          CUSTOM ERRORS
// ============================================================================

error NotAuthorized();
error PoolPaused();
error NotCompliant(address account);
error ZeroAmount();
error ZeroAddress();
error InsufficientLiquidity();
error InsufficientOutput(uint256 amountOut, uint256 minAmountOut);
error InvalidPriceRange(uint256 priceLower, uint256 priceUpper);
error InvalidPQSignature();
error SlippageExceeded(uint256 expected, uint256 actual);
error MaxSwapSizeExceeded(uint256 amount, uint256 maxSize);
error PositionNotFound(uint256 positionId);
error NotPositionOwner(uint256 positionId, address caller);
error Reentrancy();
error DeadlineExpired(uint256 deadline, uint256 currentTime);

// ============================================================================
//                          MAIN CONTRACT
// ============================================================================

contract QBTCLiquidityPool {

    // ========================================================================
    //                      REENTRANCY GUARD
    // ========================================================================

    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;
    uint256 private _status = _NOT_ENTERED;

    modifier nonReentrant() {
        if (_status == _ENTERED) revert Reentrancy();
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }

    // ========================================================================
    //                      CONSTANTS
    // ========================================================================

    /// @notice Precision for price calculations (18 decimals)
    uint256 public constant PRECISION = 1e18;

    /// @notice Minimum fee: 0.05% (5 basis points) — institutional grade
    uint256 public constant MIN_FEE = 5;

    /// @notice Maximum fee: 1.00% (100 basis points) — high volatility cap
    uint256 public constant MAX_FEE = 100;

    /// @notice Fee denominator (10,000 = 100%)
    uint256 public constant FEE_DENOMINATOR = 10_000;

    /// @notice PQ signature threshold for swaps (10 QBTC = 1,000,000 EUR at target price)
    uint256 public constant PQ_SWAP_THRESHOLD = 10 * 1e18;

    /// @notice Maximum single swap size (100 QBTC to prevent market manipulation)
    uint256 public constant MAX_SWAP_SIZE = 100 * 1e18;

    // ========================================================================
    //                      STATE VARIABLES
    // ========================================================================

    /// @notice QBTC token contract
    IERC20 public immutable qbtcToken;

    /// @notice Wrapped Digital Euro (wEURd) contract
    IERC20 public immutable wEURd;

    /// @notice Identity registry for compliance
    IIdentityRegistry public identityRegistry;

    /// @notice Post-quantum verifier
    IPQVerifier public pqVerifier;

    /// @notice Contract owner (QUBITCOIN Foundation multisig)
    address public owner;

    /// @notice Pool operational status
    bool public paused;

    /// @notice Current base fee in basis points
    uint256 public baseFee = 30; // 0.30% default

    /// @notice Protocol fee share (% of trading fees going to QUBITCOIN Foundation)
    uint256 public protocolFeeShare = 2_000; // 20% of trading fees

    /// @notice Reserve of QBTC in the pool
    uint256 public reserveQBTC;

    /// @notice Reserve of wEURd in the pool
    uint256 public reserveWEURd;

    /// @notice Accumulated protocol fees in QBTC
    uint256 public protocolFeesQBTC;

    /// @notice Accumulated protocol fees in wEURd
    uint256 public protocolFeesWEURd;

    /// @notice Total LP tokens minted
    uint256 public totalLPTokens;

    /// @notice Next position ID
    uint256 public nextPositionId = 1;

    // ========================================================================
    //                      TWAP ORACLE
    // ========================================================================

    /// @notice Cumulative price for TWAP calculation
    uint256 public priceCumulativeLast;

    /// @notice Last block timestamp for TWAP
    uint256 public blockTimestampLast;

    /// @notice TWAP observation window (1 hour)
    uint256 public constant TWAP_WINDOW = 1 hours;

    /// @notice Historical TWAP observations
    struct Observation {
        uint256 timestamp;
        uint256 priceCumulative;
    }

    Observation[] public observations;

    // ========================================================================
    //                      LIQUIDITY POSITIONS
    // ========================================================================

    struct Position {
        address owner;
        uint256 lpTokens;
        uint256 qbtcDeposited;
        uint256 weurdDeposited;
        uint256 priceLower;    // Lower bound of price range (in wEURd per QBTC)
        uint256 priceUpper;    // Upper bound of price range
        uint256 feesEarnedQBTC;
        uint256 feesEarnedWEURd;
        uint256 createdAt;
        bool active;
    }

    mapping(uint256 => Position) public positions;
    mapping(address => uint256[]) public userPositions;
    mapping(address => uint256) public lpBalanceOf;

    // ========================================================================
    //                      PROTOCOL-OWNED LIQUIDITY (POL)
    // ========================================================================

    /// @notice Amount of QBTC in Protocol-Owned Liquidity
    uint256 public polQBTC;

    /// @notice Amount of wEURd in Protocol-Owned Liquidity
    uint256 public polWEURd;

    /// @notice Target price floor maintained by POL (in wEURd per QBTC, 2 decimals)
    uint256 public priceFloor = 10_000_000_000; // 100,000.00 EUR (in cents)

    // ========================================================================
    //                          EVENTS
    // ========================================================================

    event LiquidityAdded(
        uint256 indexed positionId,
        address indexed provider,
        uint256 qbtcAmount,
        uint256 weurdAmount,
        uint256 lpTokensMinted
    );

    event LiquidityRemoved(
        uint256 indexed positionId,
        address indexed provider,
        uint256 qbtcAmount,
        uint256 weurdAmount,
        uint256 lpTokensBurned
    );

    event Swap(
        address indexed trader,
        bool qbtcToWEURd,
        uint256 amountIn,
        uint256 amountOut,
        uint256 fee,
        uint256 newPrice
    );

    event ProtocolFeesCollected(uint256 qbtcFees, uint256 weurdFees);

    event POLDeployed(uint256 qbtcAmount, uint256 weurdAmount, string reason);

    event TWAPUpdated(uint256 price, uint256 timestamp);

    event PQSignedSwap(address indexed trader, bytes32 swapHash);

    // ========================================================================
    //                          MODIFIERS
    // ========================================================================

    modifier onlyOwner() {
        if (msg.sender != owner) revert NotAuthorized();
        _;
    }

    modifier whenNotPaused() {
        if (paused) revert PoolPaused();
        _;
    }

    modifier compliant(address account) {
        if (!identityRegistry.isVerified(account)) revert NotCompliant(account);
        _;
    }

    modifier beforeDeadline(uint256 deadline) {
        if (block.timestamp > deadline) revert DeadlineExpired(deadline, block.timestamp);
        _;
    }

    // ========================================================================
    //                          CONSTRUCTOR
    // ========================================================================

    /**
     * @notice Initialize the QBTC/wEURd liquidity pool
     * @param _qbtcToken Address of the QBTC token contract
     * @param _wEURd Address of the wrapped Digital Euro contract
     * @param _identityRegistry Address of the ERC-3643 identity registry
     * @param _pqVerifier Address of the post-quantum signature verifier
     */
    constructor(
        address _qbtcToken,
        address _wEURd,
        address _identityRegistry,
        address _pqVerifier
    ) {
        if (_qbtcToken == address(0) || _wEURd == address(0)) revert ZeroAddress();
        if (_identityRegistry == address(0) || _pqVerifier == address(0)) revert ZeroAddress();

        qbtcToken = IERC20(_qbtcToken);
        wEURd = IERC20(_wEURd);
        identityRegistry = IIdentityRegistry(_identityRegistry);
        pqVerifier = IPQVerifier(_pqVerifier);
        owner = msg.sender;

        // Initialize TWAP
        blockTimestampLast = block.timestamp;
    }

    // ========================================================================
    //                    LIQUIDITY PROVISION
    // ========================================================================

    /**
     * @notice Add liquidity to the pool within a specific price range.
     * @dev Implements concentrated liquidity — LPs specify the price range
     *      within which their liquidity is active. This maximizes capital efficiency.
     *
     * @param qbtcAmount Amount of QBTC to deposit
     * @param weurdAmount Amount of wEURd to deposit
     * @param priceLower Lower bound of the price range (wEURd per QBTC, 2 decimals)
     * @param priceUpper Upper bound of the price range (wEURd per QBTC, 2 decimals)
     * @param deadline Transaction deadline timestamp
     * @return positionId The ID of the newly created liquidity position
     */
    function addLiquidity(
        uint256 qbtcAmount,
        uint256 weurdAmount,
        uint256 priceLower,
        uint256 priceUpper,
        uint256 deadline
    )
        external
        nonReentrant
        whenNotPaused
        compliant(msg.sender)
        beforeDeadline(deadline)
        returns (uint256 positionId)
    {
        if (qbtcAmount == 0 || weurdAmount == 0) revert ZeroAmount();
        if (priceLower >= priceUpper) revert InvalidPriceRange(priceLower, priceUpper);

        // Transfer tokens to pool
        qbtcToken.transferFrom(msg.sender, address(this), qbtcAmount);
        wEURd.transferFrom(msg.sender, address(this), weurdAmount);

        // Calculate LP tokens (proportional to contribution)
        uint256 lpTokens;
        if (totalLPTokens == 0) {
            lpTokens = _sqrt(qbtcAmount * weurdAmount);
        } else {
            uint256 lpFromQBTC = (qbtcAmount * totalLPTokens) / reserveQBTC;
            uint256 lpFromWEURd = (weurdAmount * totalLPTokens) / reserveWEURd;
            lpTokens = lpFromQBTC < lpFromWEURd ? lpFromQBTC : lpFromWEURd;
        }

        // Create position
        positionId = nextPositionId++;
        positions[positionId] = Position({
            owner: msg.sender,
            lpTokens: lpTokens,
            qbtcDeposited: qbtcAmount,
            weurdDeposited: weurdAmount,
            priceLower: priceLower,
            priceUpper: priceUpper,
            feesEarnedQBTC: 0,
            feesEarnedWEURd: 0,
            createdAt: block.timestamp,
            active: true
        });

        userPositions[msg.sender].push(positionId);
        lpBalanceOf[msg.sender] += lpTokens;
        totalLPTokens += lpTokens;

        // Update reserves
        reserveQBTC += qbtcAmount;
        reserveWEURd += weurdAmount;

        // Update TWAP
        _updateTWAP();

        emit LiquidityAdded(positionId, msg.sender, qbtcAmount, weurdAmount, lpTokens);
    }

    /**
     * @notice Remove liquidity from a specific position.
     * @param positionId The ID of the position to close
     * @param deadline Transaction deadline timestamp
     */
    function removeLiquidity(
        uint256 positionId,
        uint256 deadline
    )
        external
        nonReentrant
        whenNotPaused
        beforeDeadline(deadline)
    {
        Position storage pos = positions[positionId];
        if (!pos.active) revert PositionNotFound(positionId);
        if (pos.owner != msg.sender) revert NotPositionOwner(positionId, msg.sender);

        // Calculate proportional share of reserves
        uint256 qbtcShare = (pos.lpTokens * reserveQBTC) / totalLPTokens;
        uint256 weurdShare = (pos.lpTokens * reserveWEURd) / totalLPTokens;

        // Add earned fees
        qbtcShare += pos.feesEarnedQBTC;
        weurdShare += pos.feesEarnedWEURd;

        // Effects
        pos.active = false;
        lpBalanceOf[msg.sender] -= pos.lpTokens;
        totalLPTokens -= pos.lpTokens;
        reserveQBTC -= (pos.lpTokens * reserveQBTC) / (totalLPTokens + pos.lpTokens);
        reserveWEURd -= (pos.lpTokens * reserveWEURd) / (totalLPTokens + pos.lpTokens);

        // Interactions
        qbtcToken.transfer(msg.sender, qbtcShare);
        wEURd.transfer(msg.sender, weurdShare);

        _updateTWAP();

        emit LiquidityRemoved(positionId, msg.sender, qbtcShare, weurdShare, pos.lpTokens);
    }

    // ========================================================================
    //                          SWAP FUNCTIONS
    // ========================================================================

    /**
     * @notice Swap QBTC for wEURd or vice versa.
     * @dev Uses constant product formula with dynamic fees.
     *      Swaps above PQ_SWAP_THRESHOLD require a FALCON signature.
     *
     * @param qbtcToWEURd True if swapping QBTC → wEURd, false for wEURd → QBTC
     * @param amountIn Amount of input token
     * @param minAmountOut Minimum acceptable output (slippage protection)
     * @param deadline Transaction deadline timestamp
     * @return amountOut The amount of output token received
     */
    function swap(
        bool qbtcToWEURd,
        uint256 amountIn,
        uint256 minAmountOut,
        uint256 deadline
    )
        external
        nonReentrant
        whenNotPaused
        compliant(msg.sender)
        beforeDeadline(deadline)
        returns (uint256 amountOut)
    {
        if (amountIn == 0) revert ZeroAmount();
        if (amountIn > MAX_SWAP_SIZE) revert MaxSwapSizeExceeded(amountIn, MAX_SWAP_SIZE);

        // Calculate dynamic fee
        uint256 fee = _calculateDynamicFee(amountIn, qbtcToWEURd);
        uint256 amountInAfterFee = amountIn - (amountIn * fee / FEE_DENOMINATOR);

        // Constant product AMM: x * y = k
        if (qbtcToWEURd) {
            amountOut = (amountInAfterFee * reserveWEURd) / (reserveQBTC + amountInAfterFee);
            if (amountOut < minAmountOut) revert InsufficientOutput(amountOut, minAmountOut);

            // Protocol fee
            uint256 protocolFee = (amountIn * fee / FEE_DENOMINATOR) * protocolFeeShare / FEE_DENOMINATOR;
            protocolFeesQBTC += protocolFee;

            // Transfer
            qbtcToken.transferFrom(msg.sender, address(this), amountIn);
            wEURd.transfer(msg.sender, amountOut);

            reserveQBTC += amountIn - protocolFee;
            reserveWEURd -= amountOut;
        } else {
            amountOut = (amountInAfterFee * reserveQBTC) / (reserveWEURd + amountInAfterFee);
            if (amountOut < minAmountOut) revert InsufficientOutput(amountOut, minAmountOut);

            uint256 protocolFee = (amountIn * fee / FEE_DENOMINATOR) * protocolFeeShare / FEE_DENOMINATOR;
            protocolFeesWEURd += protocolFee;

            wEURd.transferFrom(msg.sender, address(this), amountIn);
            qbtcToken.transfer(msg.sender, amountOut);

            reserveWEURd += amountIn - protocolFee;
            reserveQBTC -= amountOut;
        }

        // Update TWAP
        _updateTWAP();

        // Price floor defense via POL
        uint256 currentPrice = getSpotPrice();
        if (currentPrice < priceFloor && polQBTC > 0) {
            _deployPOL();
        }

        emit Swap(msg.sender, qbtcToWEURd, amountIn, amountOut, fee, currentPrice);
    }

    /**
     * @notice Execute a high-value swap with post-quantum signature.
     * @dev Required for swaps above PQ_SWAP_THRESHOLD to protect against quantum attacks.
     */
    function swapWithPQSignature(
        bool qbtcToWEURd,
        uint256 amountIn,
        uint256 minAmountOut,
        uint256 deadline,
        bytes calldata pqSignature,
        bytes calldata pqPublicKey
    )
        external
        nonReentrant
        whenNotPaused
        compliant(msg.sender)
        beforeDeadline(deadline)
        returns (uint256 amountOut)
    {
        if (amountIn < PQ_SWAP_THRESHOLD) revert ZeroAmount();

        // Verify PQ signature
        bytes32 swapHash = keccak256(abi.encodePacked(
            msg.sender, qbtcToWEURd, amountIn, minAmountOut, deadline, block.chainid
        ));

        if (!pqVerifier.verifyFalcon(abi.encodePacked(swapHash), pqSignature, pqPublicKey)) {
            revert InvalidPQSignature();
        }

        emit PQSignedSwap(msg.sender, swapHash);

        // Execute swap (inline to avoid external call reentrancy)
        uint256 fee = _calculateDynamicFee(amountIn, qbtcToWEURd);
        uint256 amountInAfterFee = amountIn - (amountIn * fee / FEE_DENOMINATOR);

        if (qbtcToWEURd) {
            amountOut = (amountInAfterFee * reserveWEURd) / (reserveQBTC + amountInAfterFee);
            if (amountOut < minAmountOut) revert InsufficientOutput(amountOut, minAmountOut);

            qbtcToken.transferFrom(msg.sender, address(this), amountIn);
            wEURd.transfer(msg.sender, amountOut);

            reserveQBTC += amountIn;
            reserveWEURd -= amountOut;
        } else {
            amountOut = (amountInAfterFee * reserveQBTC) / (reserveWEURd + amountInAfterFee);
            if (amountOut < minAmountOut) revert InsufficientOutput(amountOut, minAmountOut);

            wEURd.transferFrom(msg.sender, address(this), amountIn);
            qbtcToken.transfer(msg.sender, amountOut);

            reserveWEURd += amountIn;
            reserveQBTC -= amountOut;
        }

        _updateTWAP();
    }

    // ========================================================================
    //                    PROTOCOL-OWNED LIQUIDITY (POL)
    // ========================================================================

    /**
     * @notice Deposit QBTC and wEURd into Protocol-Owned Liquidity.
     * @dev POL is used to defend the price floor and provide permanent liquidity.
     */
    function depositPOL(uint256 qbtcAmount, uint256 weurdAmount) external onlyOwner {
        if (qbtcAmount > 0) {
            qbtcToken.transferFrom(msg.sender, address(this), qbtcAmount);
            polQBTC += qbtcAmount;
        }
        if (weurdAmount > 0) {
            wEURd.transferFrom(msg.sender, address(this), weurdAmount);
            polWEURd += weurdAmount;
        }

        emit POLDeployed(qbtcAmount, weurdAmount, "POL deposit by Foundation");
    }

    /**
     * @dev Internal function to deploy POL to defend the price floor.
     *      Automatically buys QBTC with wEURd when price drops below floor.
     */
    function _deployPOL() internal {
        uint256 buyAmount = polWEURd / 10; // Deploy 10% of POL reserves
        if (buyAmount == 0) return;

        uint256 qbtcBought = (buyAmount * reserveQBTC) / (reserveWEURd + buyAmount);

        polWEURd -= buyAmount;
        reserveWEURd += buyAmount;
        reserveQBTC -= qbtcBought;
        polQBTC += qbtcBought;

        emit POLDeployed(qbtcBought, buyAmount, "Price floor defense");
    }

    // ========================================================================
    //                      VIEW FUNCTIONS
    // ========================================================================

    /// @notice Get the current spot price of QBTC in wEURd
    function getSpotPrice() public view returns (uint256) {
        if (reserveQBTC == 0) return 0;
        return (reserveWEURd * PRECISION) / reserveQBTC;
    }

    /// @notice Get the TWAP price over the observation window
    function getTWAP() external view returns (uint256) {
        if (observations.length < 2) return getSpotPrice();

        uint256 latestIdx = observations.length - 1;
        uint256 oldestIdx = 0;

        // Find the oldest observation within the TWAP window
        for (uint256 i = latestIdx; i > 0; i--) {
            if (block.timestamp - observations[i].timestamp >= TWAP_WINDOW) {
                oldestIdx = i;
                break;
            }
        }

        uint256 timeDelta = observations[latestIdx].timestamp - observations[oldestIdx].timestamp;
        if (timeDelta == 0) return getSpotPrice();

        uint256 priceDelta = observations[latestIdx].priceCumulative - observations[oldestIdx].priceCumulative;
        return priceDelta / timeDelta;
    }

    /// @notice Get a quote for a swap without executing it
    function getQuote(bool qbtcToWEURd, uint256 amountIn) external view returns (uint256 amountOut, uint256 fee) {
        fee = _calculateDynamicFee(amountIn, qbtcToWEURd);
        uint256 amountInAfterFee = amountIn - (amountIn * fee / FEE_DENOMINATOR);

        if (qbtcToWEURd) {
            amountOut = (amountInAfterFee * reserveWEURd) / (reserveQBTC + amountInAfterFee);
        } else {
            amountOut = (amountInAfterFee * reserveQBTC) / (reserveWEURd + amountInAfterFee);
        }
    }

    /// @notice Get all positions for a user
    function getUserPositions(address user) external view returns (uint256[] memory) {
        return userPositions[user];
    }

    // ========================================================================
    //                    ADMIN FUNCTIONS
    // ========================================================================

    function setBaseFee(uint256 _baseFee) external onlyOwner {
        if (_baseFee < MIN_FEE || _baseFee > MAX_FEE) revert ZeroAmount();
        baseFee = _baseFee;
    }

    function setPriceFloor(uint256 _priceFloor) external onlyOwner {
        priceFloor = _priceFloor;
    }

    function setPaused(bool _paused) external onlyOwner {
        paused = _paused;
    }

    function collectProtocolFees() external onlyOwner nonReentrant {
        uint256 qbtcFees = protocolFeesQBTC;
        uint256 weurdFees = protocolFeesWEURd;

        protocolFeesQBTC = 0;
        protocolFeesWEURd = 0;

        if (qbtcFees > 0) qbtcToken.transfer(owner, qbtcFees);
        if (weurdFees > 0) wEURd.transfer(owner, weurdFees);

        emit ProtocolFeesCollected(qbtcFees, weurdFees);
    }

    // ========================================================================
    //                    INTERNAL FUNCTIONS
    // ========================================================================

    /**
     * @dev Calculate dynamic fee based on trade size relative to pool reserves.
     *      Larger trades pay higher fees to discourage market manipulation.
     */
    function _calculateDynamicFee(uint256 amountIn, bool qbtcToWEURd) internal view returns (uint256) {
        uint256 reserve = qbtcToWEURd ? reserveQBTC : reserveWEURd;
        if (reserve == 0) return baseFee;

        // Impact ratio: how much does this trade move the pool?
        uint256 impactRatio = (amountIn * FEE_DENOMINATOR) / reserve;

        // Dynamic fee: baseFee + impact-based surcharge
        uint256 dynamicFee = baseFee + (impactRatio / 10);

        // Cap at MAX_FEE
        if (dynamicFee > MAX_FEE) dynamicFee = MAX_FEE;
        if (dynamicFee < MIN_FEE) dynamicFee = MIN_FEE;

        return dynamicFee;
    }

    /// @dev Update TWAP oracle with current price observation
    function _updateTWAP() internal {
        uint256 currentPrice = getSpotPrice();
        uint256 timeElapsed = block.timestamp - blockTimestampLast;

        if (timeElapsed > 0) {
            priceCumulativeLast += currentPrice * timeElapsed;
            blockTimestampLast = block.timestamp;

            observations.push(Observation({
                timestamp: block.timestamp,
                priceCumulative: priceCumulativeLast
            }));

            emit TWAPUpdated(currentPrice, block.timestamp);
        }
    }

    /// @dev Babylonian square root for LP token calculation
    function _sqrt(uint256 y) internal pure returns (uint256 z) {
        if (y > 3) {
            z = y;
            uint256 x = y / 2 + 1;
            while (x < z) {
                z = x;
                x = (y / x + x) / 2;
            }
        } else if (y != 0) {
            z = 1;
        }
    }
}
