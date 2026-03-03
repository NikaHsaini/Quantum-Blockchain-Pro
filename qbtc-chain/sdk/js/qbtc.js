/**
 * @file qbtc.js
 * @author QUBITCOIN Foundation — Nika Hsaini
 * @description Official JavaScript SDK for interacting with a QUBITCOIN (QBTC) node.
 *
 * This SDK provides a convenient interface for web applications and Node.js services to:
 *   - Connect to a QBTC node via WebSockets or HTTP
 *   - Manage post-quantum accounts (FALCON / ML-DSA / ML-KEM)
 *   - Sign and send transactions with post-quantum signatures
 *   - Interact with the Quantum Oracle and QMaaS marketplace
 *   - Interact with the CBDC / Euro Numérique bridge, liquidity pool, and router
 *   - Execute QBTC/wEURd swaps with compliance checks
 *   - Listen for on-chain quantum and CBDC events
 *
 * It is built on top of ethers.js, extending it with QBTC-specific features.
 */

const { ethers } = require("ethers");
const pqcrypto = require("./pqcrypto_lib"); // Post-quantum crypto library (Go WASM)

// ============================================================
// QBTC Provider
// ============================================================

/**
 * QBTCProvider extends ethers.JsonRpcProvider to add QBTC-specific RPC methods.
 */
class QBTCProvider extends ethers.JsonRpcProvider {
    constructor(url) {
        super(url);
    }

    /**
     * Get the result of a quantum computation job.
     * @param {string} jobId - The ID of the job (bytes32 hex string).
     * @returns {Promise<object>} The quantum job result.
     */
    async getQuantumJobResult(jobId) {
        return this.send("qbtc_getQuantumJobResult", [jobId]);
    }

    /**
     * Get the list of active quantum miners.
     * @returns {Promise<string[]>} An array of miner addresses.
     */
    async getActiveMiners() {
        return this.send("qbtc_getActiveMiners", []);
    }

    /**
     * Get the current QBTC price in wEURd from the TWAP oracle.
     * @returns {Promise<string>} The TWAP price as a string (18 decimals).
     */
    async getQBTCPrice() {
        return this.send("qbtc_getQBTCPrice", []);
    }

    /**
     * Get a swap quote for the QBTC/wEURd pool.
     * @param {boolean} qbtcToWEURd - Direction: true = sell QBTC, false = buy QBTC.
     * @param {string} amountIn - Input amount as string (18 decimals).
     * @returns {Promise<object>} The swap quote { amountOut, fee, spotPrice }.
     */
    async getSwapQuote(qbtcToWEURd, amountIn) {
        return this.send("qbtc_getSwapQuote", [qbtcToWEURd, amountIn]);
    }
}

// ============================================================
// QBTC Wallet (Post-Quantum)
// ============================================================

/**
 * QBTCWallet represents a QBTC account with a post-quantum (ML-DSA) key pair.
 * It extends ethers.Wallet to override signing methods with PQ algorithms.
 */
class QBTCWallet extends ethers.Wallet {
    /**
     * Creates a new QBTCWallet instance.
     * @param {object} privateKey - The ML-DSA private key object.
     * @param {QBTCProvider} provider - The QBTC provider.
     */
    constructor(privateKey, provider) {
        super(ethers.hexlify(ethers.randomBytes(32)), provider);
        this.mldsaPrivateKey = privateKey;
        this.mldsaPublicKey = pqcrypto.getPublicKey(privateKey);
        this.address = this.getAddressFromPublicKey(this.mldsaPublicKey);
    }

    /**
     * Creates a new random QBTC wallet.
     * @param {QBTCProvider} provider - The QBTC provider.
     * @returns {QBTCWallet} A new wallet instance.
     */
    static createRandom(provider) {
        const privateKey = pqcrypto.generateMLDSAKeyPair();
        return new QBTCWallet(privateKey, provider);
    }

    /**
     * Derives the QBTC address from an ML-DSA public key.
     * @param {object} publicKey - The ML-DSA public key.
     * @returns {string} The QBTC address (last 20 bytes of keccak256(pubKey)).
     */
    getAddressFromPublicKey(publicKey) {
        const pkBytes = pqcrypto.serializePublicKey(publicKey);
        const hash = ethers.keccak256(pkBytes);
        return ethers.getAddress(ethers.dataSlice(hash, 12));
    }

    /**
     * Signs a transaction with the wallet's ML-DSA private key.
     * @param {object} tx - The transaction object.
     * @returns {Promise<string>} The signed transaction hex string.
     */
    async signTransaction(tx) {
        const txHash = this.computeTxHash(tx);
        const signature = pqcrypto.sign(this.mldsaPrivateKey, txHash);
        const serializedSignature = pqcrypto.serializeSignature(signature);
        tx.pqSignature = ethers.hexlify(serializedSignature);
        return this.serializeTx(tx);
    }

    /**
     * Computes the hash of a transaction to be signed.
     * @param {object} tx - The transaction object.
     * @returns {Uint8Array} The 32-byte transaction hash.
     */
    computeTxHash(tx) {
        const to = tx.to ? ethers.getAddress(tx.to) : "0x";
        const data = tx.data ? ethers.hexlify(tx.data) : "0x";
        const value = tx.value ? ethers.toBigInt(tx.value) : 0n;
        const nonce = tx.nonce ? tx.nonce : 0;

        const encoded = ethers.solidityPacked(
            ["uint256", "address", "uint256", "bytes"],
            [nonce, to, value, data]
        );

        return ethers.keccak256(encoded);
    }

    /**
     * Serializes a QBTC transaction including the post-quantum signature.
     * @param {object} tx - The transaction object.
     * @returns {string} The serialized transaction hex string.
     */
    serializeTx(tx) {
        // In production: RLP encoding with custom QBTC transaction type
        return JSON.stringify(tx);
    }
}

// ============================================================
// Contract ABIs
// ============================================================

const quantumOracleAbi = [
    "function submitJob(uint256 numQubits, bytes32 circuitHash, uint256 deadline, uint8 backend, uint256 resilienceLevel) external payable returns (bytes32 jobId)",
    "function groverSearch(uint256 numQubits, uint256 targetState) external returns (uint256 foundState)",
    "function verifyPostQuantumSignature(bytes32 messageHash, bytes calldata publicKey, bytes calldata signature) external returns (bool isValid)",
    "function claimRewards() external",
    "event QuantumJobSubmitted(bytes32 indexed jobId, address indexed submitter, uint256 numQubits, uint256 reward, uint256 deadline, uint8 backend)",
    "event QuantumJobCompleted(bytes32 indexed jobId, address indexed miner, bytes32 resultHash, uint256 reward)",
];

const euroDigitalBridgeAbi = [
    "function mint(address recipient, uint256 amount, bytes32 dl3sTransactionId) external",
    "function burn(uint256 amount, address settlementAgent, bytes32 dl3sTransactionId) external",
    "function balanceOf(address account) external view returns (uint256)",
    "function getHoldingLimit(address account) external view returns (uint256)",
    "function isKYCVerified(address account) external view returns (bool)",
    "event WEURdMinted(address indexed recipient, uint256 amount, bytes32 indexed dl3sTransactionId)",
    "event WEURdBurned(address indexed burner, uint256 amount, bytes32 indexed dl3sTransactionId)",
];

const liquidityPoolAbi = [
    "function swapQBTCForWEURd(uint256 amountIn, uint256 minAmountOut, uint256 deadline) external returns (uint256 amountOut)",
    "function swapWEURdForQBTC(uint256 amountIn, uint256 minAmountOut, uint256 deadline) external returns (uint256 amountOut)",
    "function addLiquidity(uint256 qbtcAmount, uint256 weurdAmount, int24 tickLower, int24 tickUpper) external returns (uint256 positionId)",
    "function removeLiquidity(uint256 positionId) external returns (uint256 qbtcAmount, uint256 weurdAmount)",
    "function getSpotPrice() external view returns (uint256)",
    "function getTWAP() external view returns (uint256)",
    "function getReserves() external view returns (uint256 reserveQBTC, uint256 reserveWEURd)",
    "event Swap(address indexed trader, bool qbtcToWEURd, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 spotPrice)",
];

const cbdcRouterAbi = [
    "function routeSwap(bytes32 fromAsset, bytes32 toAsset, uint256 amountIn, uint256 minAmountOut, address recipient, uint256 deadline) external returns (uint256 amountOut)",
    "function getRoute(bytes32 fromAsset, bytes32 toAsset) external view returns (address[] memory path)",
    "function getSupportedCBDCs() external view returns (bytes32[] memory)",
    "event CBDCSwapExecuted(bytes32 indexed fromAsset, bytes32 indexed toAsset, uint256 amountIn, uint256 amountOut, address indexed recipient)",
];

// ============================================================
// Quantum Oracle Interaction
// ============================================================

/**
 * QuantumOracle provides an interface to the QuantumOracle smart contract.
 */
class QuantumOracle {
    /**
     * @param {string} address - The contract address.
     * @param {QBTCWallet} wallet - The QBTC wallet to use for transactions.
     */
    constructor(address, wallet) {
        this.contract = new ethers.Contract(address, quantumOracleAbi, wallet);
    }

    /**
     * Submits a quantum computation job.
     * @param {number} numQubits - Number of qubits.
     * @param {string} circuitHash - Hash of the circuit definition (bytes32 hex).
     * @param {number} deadline - Block number deadline.
     * @param {number} backend - Quantum backend (0=LOCAL, 1=IBM_EAGLE, 2=IBM_HERON).
     * @param {number} resilienceLevel - IBM Qiskit resilience level (0-2).
     * @param {string} reward - Reward in QBTC (e.g., "1.0").
     * @returns {Promise<ethers.TransactionResponse>} The transaction response.
     */
    async submitJob(numQubits, circuitHash, deadline, backend, resilienceLevel, reward) {
        const rewardWei = ethers.parseEther(reward);
        return this.contract.submitJob(numQubits, circuitHash, deadline, backend, resilienceLevel, {
            value: rewardWei,
        });
    }

    async groverSearch(numQubits, targetState) {
        return this.contract.groverSearch(numQubits, targetState);
    }

    async verifyPostQuantumSignature(messageHash, publicKey, signature) {
        return this.contract.verifyPostQuantumSignature(
            messageHash,
            ethers.hexlify(publicKey),
            ethers.hexlify(signature)
        );
    }

    async claimRewards() {
        return this.contract.claimRewards();
    }
}

// ============================================================
// Euro Digital Bridge (CBDC)
// ============================================================

/**
 * EuroDigitalBridge provides an interface to the EuroDigitalBridge smart contract.
 * Enables minting/burning of wEURd (wrapped Digital Euro) for authorized settlement agents.
 */
class EuroDigitalBridge {
    /**
     * @param {string} address - The contract address.
     * @param {QBTCWallet} wallet - The QBTC wallet to use for transactions.
     */
    constructor(address, wallet) {
        this.contract = new ethers.Contract(address, euroDigitalBridgeAbi, wallet);
    }

    /**
     * Mint wEURd (only for authorized settlement agents).
     * @param {string} recipient - Recipient address.
     * @param {string} amount - Amount in EUR (e.g., "1000.00").
     * @param {string} dl3sTransactionId - DL3S/TARGET2 settlement reference (bytes32 hex).
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async mint(recipient, amount, dl3sTransactionId) {
        const amountWei = ethers.parseUnits(amount, 2); // 2 decimals for EUR
        return this.contract.mint(recipient, amountWei, dl3sTransactionId);
    }

    /**
     * Burn wEURd and initiate settlement in Digital Euro.
     * @param {string} amount - Amount in EUR (e.g., "1000.00").
     * @param {string} settlementAgent - Settlement agent address.
     * @param {string} dl3sTransactionId - DL3S/TARGET2 reference (bytes32 hex).
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async burn(amount, settlementAgent, dl3sTransactionId) {
        const amountWei = ethers.parseUnits(amount, 2);
        return this.contract.burn(amountWei, settlementAgent, dl3sTransactionId);
    }

    /**
     * Get the wEURd balance of an account.
     * @param {string} account - Account address.
     * @returns {Promise<string>} Balance formatted in EUR.
     */
    async balanceOf(account) {
        const balance = await this.contract.balanceOf(account);
        return ethers.formatUnits(balance, 2);
    }

    /**
     * Check if an account is KYC-verified for CBDC operations.
     * @param {string} account - Account address.
     * @returns {Promise<boolean>}
     */
    async isKYCVerified(account) {
        return this.contract.isKYCVerified(account);
    }

    /**
     * Get the holding limit for an account (3,000 EUR retail, 1,000,000 EUR institutional).
     * @param {string} account - Account address.
     * @returns {Promise<string>} Holding limit formatted in EUR.
     */
    async getHoldingLimit(account) {
        const limit = await this.contract.getHoldingLimit(account);
        return ethers.formatUnits(limit, 2);
    }
}

// ============================================================
// QBTC Liquidity Pool (AMM)
// ============================================================

/**
 * QBTCLiquidityPool provides an interface to the QBTCLiquidityPool smart contract.
 * Supports QBTC/wEURd swaps with algorithmic liquidity management.
 */
class QBTCLiquidityPool {
    /**
     * @param {string} address - The contract address.
     * @param {QBTCWallet} wallet - The QBTC wallet to use for transactions.
     */
    constructor(address, wallet) {
        this.contract = new ethers.Contract(address, liquidityPoolAbi, wallet);
    }

    /**
     * Swap QBTC for wEURd.
     * @param {string} amountIn - Amount of QBTC to sell (e.g., "0.5").
     * @param {string} minAmountOut - Minimum wEURd to receive (slippage protection).
     * @param {number} deadline - Block timestamp deadline.
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async swapQBTCForWEURd(amountIn, minAmountOut, deadline) {
        return this.contract.swapQBTCForWEURd(
            ethers.parseEther(amountIn),
            ethers.parseEther(minAmountOut),
            deadline
        );
    }

    /**
     * Swap wEURd for QBTC.
     * @param {string} amountIn - Amount of wEURd to spend.
     * @param {string} minAmountOut - Minimum QBTC to receive.
     * @param {number} deadline - Block timestamp deadline.
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async swapWEURdForQBTC(amountIn, minAmountOut, deadline) {
        return this.contract.swapWEURdForQBTC(
            ethers.parseEther(amountIn),
            ethers.parseEther(minAmountOut),
            deadline
        );
    }

    /**
     * Add concentrated liquidity to the pool.
     * @param {string} qbtcAmount - QBTC amount.
     * @param {string} weurdAmount - wEURd amount.
     * @param {number} tickLower - Lower price tick.
     * @param {number} tickUpper - Upper price tick.
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async addLiquidity(qbtcAmount, weurdAmount, tickLower, tickUpper) {
        return this.contract.addLiquidity(
            ethers.parseEther(qbtcAmount),
            ethers.parseEther(weurdAmount),
            tickLower,
            tickUpper
        );
    }

    /**
     * Remove liquidity from a position.
     * @param {number} positionId - LP position ID.
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async removeLiquidity(positionId) {
        return this.contract.removeLiquidity(positionId);
    }

    /**
     * Get the current spot price (wEURd per QBTC).
     * @returns {Promise<string>} Spot price formatted.
     */
    async getSpotPrice() {
        const price = await this.contract.getSpotPrice();
        return ethers.formatEther(price);
    }

    /**
     * Get the TWAP price (Time-Weighted Average Price).
     * @returns {Promise<string>} TWAP price formatted.
     */
    async getTWAP() {
        const twap = await this.contract.getTWAP();
        return ethers.formatEther(twap);
    }

    /**
     * Get the pool reserves.
     * @returns {Promise<{reserveQBTC: string, reserveWEURd: string}>}
     */
    async getReserves() {
        const [reserveQBTC, reserveWEURd] = await this.contract.getReserves();
        return {
            reserveQBTC: ethers.formatEther(reserveQBTC),
            reserveWEURd: ethers.formatEther(reserveWEURd),
        };
    }
}

// ============================================================
// CBDC Router (Multi-CBDC Interoperability)
// ============================================================

/**
 * CBDCRouter provides cross-CBDC routing for swaps between QBTC, wEURd,
 * and other European CBDC/stablecoin representations.
 */
class CBDCRouter {
    /**
     * @param {string} address - The contract address.
     * @param {QBTCWallet} wallet - The QBTC wallet to use for transactions.
     */
    constructor(address, wallet) {
        this.contract = new ethers.Contract(address, cbdcRouterAbi, wallet);
    }

    /**
     * Execute a cross-CBDC swap.
     * @param {string} fromAsset - Source CBDC identifier (bytes32 hex).
     * @param {string} toAsset - Destination CBDC identifier (bytes32 hex).
     * @param {string} amountIn - Input amount.
     * @param {string} minAmountOut - Minimum output (slippage protection).
     * @param {string} recipient - Recipient address.
     * @param {number} deadline - Block timestamp deadline.
     * @returns {Promise<ethers.TransactionResponse>}
     */
    async routeSwap(fromAsset, toAsset, amountIn, minAmountOut, recipient, deadline) {
        return this.contract.routeSwap(
            fromAsset,
            toAsset,
            ethers.parseEther(amountIn),
            ethers.parseEther(minAmountOut),
            recipient,
            deadline
        );
    }

    /**
     * Get the routing path between two CBDC assets.
     * @param {string} fromAsset - Source CBDC identifier.
     * @param {string} toAsset - Destination CBDC identifier.
     * @returns {Promise<string[]>} Array of intermediate contract addresses.
     */
    async getRoute(fromAsset, toAsset) {
        return this.contract.getRoute(fromAsset, toAsset);
    }

    /**
     * Get all supported CBDC identifiers.
     * @returns {Promise<string[]>} Array of CBDC identifiers (bytes32 hex).
     */
    async getSupportedCBDCs() {
        return this.contract.getSupportedCBDCs();
    }
}

// ============================================================
// Post-Quantum Crypto Library (WASM placeholder)
// ============================================================

// In production, this is a WebAssembly module compiled from the Go
// pqcrypto package (qbtc-chain/crypto/pqcrypto), exposing FALCON-1024,
// ML-DSA-65, ML-KEM-1024 and SHA-999 to the browser and Node.js.
// The Go WASM build is fully compatible with the Ethereum/go-ethereum stack.
const pqcrypto_lib = {
    generateMLDSAKeyPair: () => ({ secret: "...", public: "..." }),
    getPublicKey: (privKey) => privKey.public,
    serializePublicKey: (pubKey) => new Uint8Array(1952),
    sign: (privKey, hash) => ({ r: "...", s: "..." }),
    serializeSignature: (sig) => new Uint8Array(3309),
};

// ============================================================
// Module Exports
// ============================================================

module.exports = {
    QBTCProvider,
    QBTCWallet,
    QuantumOracle,
    EuroDigitalBridge,
    QBTCLiquidityPool,
    CBDCRouter,
};

// ============================================================
// Example Usage
// ============================================================

async function main() {
    console.log("Connecting to QBTC node...");
    const provider = new QBTCProvider("http://localhost:8545");

    console.log("Creating a new post-quantum wallet...");
    const wallet = QBTCWallet.createRandom(provider);
    console.log("Wallet Address:", wallet.address);

    const balance = await provider.getBalance(wallet.address);
    console.log("Wallet Balance:", ethers.formatEther(balance), "QBTC");

    // --- CBDC Operations ---

    console.log("\n--- Euro Numérique / CBDC Operations ---");

    const bridgeAddress = "0x1000000000000000000000000000000000000001";
    const poolAddress = "0x1000000000000000000000000000000000000002";
    const routerAddress = "0x1000000000000000000000000000000000000003";

    const bridge = new EuroDigitalBridge(bridgeAddress, wallet);
    const pool = new QBTCLiquidityPool(poolAddress, wallet);
    const router = new CBDCRouter(routerAddress, wallet);

    // Check KYC status
    const isKYC = await bridge.isKYCVerified(wallet.address);
    console.log("KYC Verified:", isKYC);

    // Get wEURd balance
    const weurdBalance = await bridge.balanceOf(wallet.address);
    console.log("wEURd Balance:", weurdBalance, "EUR");

    // Get QBTC/wEURd spot price and TWAP
    const spotPrice = await pool.getSpotPrice();
    const twapPrice = await pool.getTWAP();
    console.log("QBTC Spot Price:", spotPrice, "wEURd");
    console.log("QBTC TWAP Price:", twapPrice, "wEURd");

    // Get pool reserves
    const reserves = await pool.getReserves();
    console.log("Pool Reserves:", reserves.reserveQBTC, "QBTC /", reserves.reserveWEURd, "wEURd");

    // Example: Swap wEURd for QBTC
    try {
        console.log("\nSwapping wEURd for QBTC...");
        const deadline = Math.floor(Date.now() / 1000) + 3600; // 1 hour
        const tx = await pool.swapWEURdForQBTC("50000", "0.4", deadline);
        console.log("Swap transaction sent:", tx.hash);
    } catch (error) {
        console.error("Swap failed:", error.message);
    }

    // --- Quantum Oracle ---

    console.log("\n--- Quantum Oracle Operations ---");
    const oracleAddress = "0x1000000000000000000000000000000000000005";
    const oracle = new QuantumOracle(oracleAddress, wallet);

    try {
        console.log("Submitting a quantum job on IBM Heron...");
        const tx = await oracle.submitJob(
            8,                              // 8 qubits
            ethers.randomBytes(32),         // circuit hash
            10000,                          // deadline
            2,                              // IBM_HERON backend
            1,                              // resilience level 1
            "1.0"                           // 1 QBTC reward
        );
        console.log("Job submission transaction sent:", tx.hash);
    } catch (error) {
        console.error("Failed to submit job:", error.message);
    }

    // List supported CBDCs
    try {
        const cbdcs = await router.getSupportedCBDCs();
        console.log("\nSupported CBDCs:", cbdcs);
    } catch (error) {
        console.error("Failed to list CBDCs:", error.message);
    }
}

if (require.main === module) {
    main().catch(console.error);
}
