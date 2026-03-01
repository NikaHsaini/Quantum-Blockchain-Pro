/**
 * @file qbp.js
 * @author Quantum Blockchain Pro Team
 * @description Official JavaScript SDK for interacting with a Quantum Blockchain Pro (QBP) node.
 *
 * This SDK provides a convenient interface for web applications and Node.js services to:
 *   - Connect to a QBP node via WebSockets or HTTP
 *   - Manage post-quantum accounts (ML-DSA/ML-KEM)
 *   - Sign and send transactions with ML-DSA signatures
 *   - Interact with the Quantum Oracle and QMaaS marketplace
 *   - Listen for on-chain quantum events
 *
 * It is built on top of ethers.js, extending it with QBP-specific features.
 */

const { ethers } = require("ethers");
const pqcrypto = require("./pqcrypto_lib"); // Placeholder for post-quantum crypto library

// ============================================================
// QBP Provider
// ============================================================

/**
 * QBPProvider extends ethers.JsonRpcProvider to add QBP-specific RPC methods.
 */
class QBPProvider extends ethers.JsonRpcProvider {
    constructor(url) {
        super(url);
    }

    /**
     * Get the result of a quantum computation job.
     * @param {string} jobId - The ID of the job (bytes32 hex string).
     * @returns {Promise<object>} The quantum job result.
     */
    async getQuantumJobResult(jobId) {
        return this.send("qbp_getQuantumJobResult", [jobId]);
    }

    /**
     * Get the list of active quantum miners.
     * @returns {Promise<string[]>} An array of miner addresses.
     */
    async getActiveMiners() {
        return this.send("qbp_getActiveMiners", []);
    }
}

// ============================================================
// QBP Wallet (Post-Quantum)
// ============================================================

/**
 * QBPWallet represents a QBP account with a post-quantum (ML-DSA) key pair.
 * It extends ethers.Wallet to override signing methods.
 */
class QBPWallet extends ethers.Wallet {
    /**
     * Creates a new QBPWallet instance.
     * @param {object} privateKey - The ML-DSA private key object.
     * @param {QBPProvider} provider - The QBP provider.
     */
    constructor(privateKey, provider) {
        // Pass a dummy private key to the parent constructor
        super(ethers.hexlify(ethers.randomBytes(32)), provider);

        this.mldsaPrivateKey = privateKey;
        this.mldsaPublicKey = pqcrypto.getPublicKey(privateKey);
        this.address = this.getAddressFromPublicKey(this.mldsaPublicKey);
    }

    /**
     * Creates a new random QBP wallet.
     * @param {QBPProvider} provider - The QBP provider.
     * @returns {QBPWallet} A new wallet instance.
     */
    static createRandom(provider) {
        const privateKey = pqcrypto.generateMLDSAKeyPair();
        return new QBPWallet(privateKey, provider);
    }

    /**
     * Derives the QBP address from an ML-DSA public key.
     * @param {object} publicKey - The ML-DSA public key.
     * @returns {string} The QBP address.
     */
    getAddressFromPublicKey(publicKey) {
        const pkBytes = pqcrypto.serializePublicKey(publicKey);
        const hash = ethers.keccak256(pkBytes);
        return ethers.getAddress(ethers.dataSlice(hash, 12)); // Last 20 bytes
    }

    /**
     * Signs a transaction with the wallet's ML-DSA private key.
     * @param {object} tx - The transaction object.
     * @returns {Promise<string>} The signed transaction hex string.
     */
    async signTransaction(tx) {
        // In QBP, the signature replaces the v, r, s fields.
        // The transaction hash to be signed is computed differently.
        const txHash = this.computeTxHash(tx);

        const signature = pqcrypto.sign(this.mldsaPrivateKey, txHash);
        const serializedSignature = pqcrypto.serializeSignature(signature);

        // Add the post-quantum signature to the transaction
        tx.pqSignature = ethers.hexlify(serializedSignature);

        // Serialize the transaction with the PQ signature
        return this.serializeTx(tx);
    }

    /**
     * Computes the hash of a transaction to be signed.
     * @param {object} tx - The transaction object.
     * @returns {Uint8Array} The 32-byte transaction hash.
     */
    computeTxHash(tx) {
        // Simplified hashing for example
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
     * Serializes a QBP transaction including the post-quantum signature.
     * @param {object} tx - The transaction object.
     * @returns {string} The serialized transaction hex string.
     */
    serializeTx(tx) {
        // In a real implementation, this would use RLP encoding
        // with a custom transaction type for QBP.
        return JSON.stringify(tx);
    }
}

// ============================================================
// Quantum Oracle Interaction
// ============================================================

const quantumOracleAbi = [
    "function submitJob(uint256 numQubits, bytes32 circuitHash, uint256 deadline) external payable returns (bytes32 jobId)",
    "function groverSearch(uint256 numQubits, uint256 targetState) external returns (uint256 foundState)",
    "function verifyPostQuantumSignature(bytes32 messageHash, bytes calldata publicKey, bytes calldata signature) external returns (bool isValid)",
    "event QuantumJobSubmitted(bytes32 indexed jobId, address indexed submitter, uint256 reward)",
];

/**
 * QuantumOracle provides an interface to the QuantumOracle smart contract.
 */
class QuantumOracle {
    /**
     * Creates a new QuantumOracle instance.
     * @param {string} address - The contract address.
     * @param {QBPWallet} wallet - The QBP wallet to use for transactions.
     */
    constructor(address, wallet) {
        this.contract = new ethers.Contract(address, quantumOracleAbi, wallet);
    }

    /**
     * Submits a quantum computation job.
     * @param {number} numQubits - Number of qubits.
     * @param {string} circuitHash - Hash of the circuit definition (bytes32 hex).
     * @param {number} deadline - Block number deadline.
     * @param {string} reward - Reward in QBP (e.g., "100.0").
     * @returns {Promise<ethers.TransactionResponse>} The transaction response.
     */
    async submitJob(numQubits, circuitHash, deadline, reward) {
        const rewardWei = ethers.parseEther(reward);
        return this.contract.submitJob(numQubits, circuitHash, deadline, {
            value: rewardWei,
        });
    }

    /**
     * Executes Grover's search algorithm on-chain.
     * @param {number} numQubits - Number of qubits.
     * @param {number} targetState - The state to search for.
     * @returns {Promise<bigint>} The found state.
     */
    async groverSearch(numQubits, targetState) {
        return this.contract.groverSearch(numQubits, targetState);
    }

    /**
     * Verifies an ML-DSA signature on-chain.
     * @param {string} messageHash - The hash of the message (bytes32 hex).
     * @param {Uint8Array} publicKey - The ML-DSA public key bytes.
     * @param {Uint8Array} signature - The ML-DSA signature bytes.
     * @returns {Promise<boolean>} True if the signature is valid.
     */
    async verifyPostQuantumSignature(messageHash, publicKey, signature) {
        return this.contract.verifyPostQuantumSignature(
            messageHash,
            ethers.hexlify(publicKey),
            ethers.hexlify(signature)
        );
    }
}

// ============================================================
// Example Usage
// ============================================================

async function main() {
    console.log("Connecting to QBP node...");
    const provider = new QBPProvider("http://localhost:8545");

    console.log("Creating a new post-quantum wallet...");
    const wallet = QBPWallet.createRandom(provider);
    console.log("Wallet Address:", wallet.address);

    const balance = await provider.getBalance(wallet.address);
    console.log("Wallet Balance:", ethers.formatEther(balance), "QBP");

    console.log("\nInteracting with Quantum Oracle...");
    const oracleAddress = "0x..."; // Deployed QuantumOracle contract address
    const oracle = new QuantumOracle(oracleAddress, wallet);

    // Example: Submit a quantum job
    try {
        console.log("Submitting a quantum job...");
        const tx = await oracle.submitJob(8, ethers.randomBytes(32), 10000, "50.0");
        console.log("Job submission transaction sent:", tx.hash);
        const receipt = await tx.wait();
        console.log("Transaction confirmed in block:", receipt.blockNumber);
    } catch (error) {
        console.error("Failed to submit job:", error.message);
    }

    // Example: Verify a post-quantum signature
    try {
        console.log("\nVerifying an ML-DSA signature on-chain...");
        const message = "Hello, Quantum World!";
        const messageHash = ethers.id(message);

        const signature = pqcrypto.sign(wallet.mldsaPrivateKey, ethers.getBytes(messageHash));
        const serializedSig = pqcrypto.serializeSignature(signature);

        const isValid = await oracle.verifyPostQuantumSignature(
            messageHash,
            pqcrypto.serializePublicKey(wallet.mldsaPublicKey),
            serializedSig
        );
        console.log("Is the signature valid?", isValid);
    } catch (error) {
        console.error("Failed to verify signature:", error.message);
    }
}

// Dummy pqcrypto library for demonstration.
// In the production implementation, this is a WebAssembly module
// compiled from the Go pqcrypto package (qbp-chain/crypto/pqcrypto),
// exposing FALCON-1024, ML-DSA-65, ML-KEM and SHA-999 to the browser.
// The Go WASM build is fully compatible with the Ethereum/go-ethereum stack.
const pqcrypto_lib = {
    generateMLDSAKeyPair: () => ({ secret: "...", public: "..." }),
    getPublicKey: (privKey) => privKey.public,
    serializePublicKey: (pubKey) => new Uint8Array(1952),
    sign: (privKey, hash) => ({ r: "...", s: "..." }),
    serializeSignature: (sig) => new Uint8Array(3309),
};

// Run example if the script is executed directly
if (require.main === module) {
    main().catch(console.error);
}
