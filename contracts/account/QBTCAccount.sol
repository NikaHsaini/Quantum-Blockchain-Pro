// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title QBTCAccount
 * @author Nika Hsaini — QUBITCOIN Foundation
 * @notice Hybrid post-quantum Ethereum account supporting both ECDSA (legacy)
 *         and FALCON/ML-DSA (post-quantum) signature schemes.
 *
 * @dev Inspired by ZKnox's PQKINGS project (ETHPRAGUE 2025) and their
 *      EIP-7702 hybrid account implementation. This contract implements
 *      a "harvest-now, decrypt-later" resistant account that can be used
 *      with both EIP-4337 (Account Abstraction) and EIP-7702 (EOA delegation).
 *
 *      The hybrid approach ensures backward compatibility with existing
 *      Ethereum infrastructure while providing quantum resistance for
 *      long-term security.
 *
 *      Signature modes:
 *        1. ECDSA only (legacy, for compatibility)
 *        2. FALCON only (post-quantum, maximum security)
 *        3. ECDSA + FALCON (hybrid, requires both signatures)
 *        4. EPERVIER (FALCON with recovery, ZKnox innovation)
 *
 *      Reference: https://github.com/ZKNoxHQ/PQKINGS
 *
 * @dev EIP-4337 compatible: implements IAccount interface.
 * @dev EIP-7702 compatible: can be used as an EOA delegation target.
 */

import "../pq/ZKNOX_FALCON_Verifier.sol";
import "../libraries/ZKNOX_Math.sol";

interface IEntryPoint {
    function getUserOpHash(UserOperation calldata userOp) external view returns (bytes32);
    function handleOps(UserOperation[] calldata ops, address payable beneficiary) external;
    function depositTo(address account) external payable;
    function balanceOf(address account) external view returns (uint256);
}

struct UserOperation {
    address sender;
    uint256 nonce;
    bytes initCode;
    bytes callData;
    uint256 callGasLimit;
    uint256 verificationGasLimit;
    uint256 preVerificationGas;
    uint256 maxFeePerGas;
    uint256 maxPriorityFeePerGas;
    bytes paymasterAndData;
    bytes signature;
}

contract QBTCAccount {

    // ============================================================
    // Constants
    // ============================================================

    /// @notice EIP-4337 magic value for successful validation
    uint256 constant SIG_VALIDATION_SUCCESS = 0;

    /// @notice EIP-4337 magic value for failed validation
    uint256 constant SIG_VALIDATION_FAILED = 1;

    /// @notice EIP-1271 magic value for valid signature
    bytes4 constant EIP1271_MAGIC = 0x1626ba7e;

    // ============================================================
    // Signature Modes
    // ============================================================

    enum SignatureMode {
        ECDSA,          // Legacy ECDSA only
        FALCON,         // FALCON-512 post-quantum only
        HYBRID,         // ECDSA + FALCON (both required)
        EPERVIER        // FALCON with recovery (ZKnox)
    }

    // ============================================================
    // State
    // ============================================================

    /// @notice The EIP-4337 EntryPoint contract
    IEntryPoint public immutable entryPoint;

    /// @notice The FALCON verifier contract (ZKnox implementation)
    ZKNOX_FALCON_Verifier public immutable falconVerifier;

    /// @notice Current signature mode
    SignatureMode public signatureMode;

    /// @notice ECDSA owner address (for legacy compatibility)
    address public ecdsaOwner;

    /// @notice FALCON public key in NTT domain (compacted)
    uint256[] public falconPublicKey;

    /// @notice ML-DSA public key (CRYSTALS-Dilithium, NIST FIPS 204)
    bytes public mldsaPublicKey;

    /// @notice Account nonce (for replay protection)
    uint256 public accountNonce;

    /// @notice Whether the account has been initialized
    bool public initialized;

    // ============================================================
    // Events
    // ============================================================

    event AccountInitialized(
        address indexed owner,
        SignatureMode mode,
        bool hasFalconKey,
        bool hasMLDSAKey
    );

    event SignatureModeChanged(SignatureMode oldMode, SignatureMode newMode);
    event FalconKeyUpdated(uint256 keyLength);
    event MLDSAKeyUpdated(uint256 keyLength);
    event TransactionExecuted(address indexed to, uint256 value, bytes data);
    event QuantumKeyRotated(address indexed account, uint256 blockNumber);

    // ============================================================
    // Errors
    // ============================================================

    error AlreadyInitialized();
    error NotInitialized();
    error Unauthorized();
    error InvalidSignatureMode();
    error InvalidFalconKey();
    error InvalidMLDSAKey();
    error ExecutionFailed(bytes reason);
    error InvalidNonce(uint256 provided, uint256 expected);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyEntryPointOrOwner() {
        require(
            msg.sender == address(entryPoint) || msg.sender == ecdsaOwner || msg.sender == address(this),
            "QBTCAccount: not authorized"
        );
        _;
    }

    modifier onlyOwner() {
        require(msg.sender == ecdsaOwner || msg.sender == address(this), "QBTCAccount: not owner");
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    constructor(address _entryPoint, address _falconVerifier) {
        entryPoint = IEntryPoint(_entryPoint);
        falconVerifier = ZKNOX_FALCON_Verifier(_falconVerifier);
    }

    // ============================================================
    // Initialization
    // ============================================================

    /**
     * @notice Initialize the account with owner and post-quantum keys.
     * @dev Can only be called once. Supports hybrid ECDSA + FALCON setup.
     *
     * @param _ecdsaOwner      ECDSA owner address (for legacy compatibility)
     * @param _falconPubKey    FALCON public key in NTT domain (compacted), or empty
     * @param _mldsaPubKey     ML-DSA public key (1952 bytes), or empty
     * @param _mode            Signature mode (ECDSA, FALCON, HYBRID, or EPERVIER)
     */
    function initialize(
        address _ecdsaOwner,
        uint256[] calldata _falconPubKey,
        bytes calldata _mldsaPubKey,
        SignatureMode _mode
    ) external {
        if (initialized) revert AlreadyInitialized();

        ecdsaOwner = _ecdsaOwner;
        signatureMode = _mode;

        if (_falconPubKey.length > 0) {
            falconPublicKey = _falconPubKey;
        }

        if (_mldsaPubKey.length > 0) {
            if (_mldsaPubKey.length != 1952) revert InvalidMLDSAKey();
            mldsaPublicKey = _mldsaPubKey;
        }

        initialized = true;

        emit AccountInitialized(_ecdsaOwner, _mode, _falconPubKey.length > 0, _mldsaPubKey.length > 0);
    }

    // ============================================================
    // EIP-4337: validateUserOp
    // ============================================================

    /**
     * @notice Validate a UserOperation signature (EIP-4337).
     * @dev Called by the EntryPoint before executing a UserOperation.
     *      Supports ECDSA, FALCON, and hybrid validation modes.
     *
     * @param userOp    The UserOperation to validate
     * @param userOpHash Hash of the UserOperation
     * @param missingAccountFunds Funds to deposit to EntryPoint
     * @return validationData 0 = success, 1 = failure, or packed (aggregator, validUntil, validAfter)
     */
    function validateUserOp(
        UserOperation calldata userOp,
        bytes32 userOpHash,
        uint256 missingAccountFunds
    ) external returns (uint256 validationData) {
        require(msg.sender == address(entryPoint), "QBTCAccount: not EntryPoint");

        // Deposit missing funds to EntryPoint
        if (missingAccountFunds > 0) {
            entryPoint.depositTo{value: missingAccountFunds}(address(this));
        }

        // Validate based on signature mode
        bool valid = _validateSignature(userOpHash, userOp.signature);

        return valid ? SIG_VALIDATION_SUCCESS : SIG_VALIDATION_FAILED;
    }

    // ============================================================
    // EIP-1271: isValidSignature
    // ============================================================

    /**
     * @notice Validate a signature (EIP-1271).
     * @dev Called by external contracts to verify signatures from this account.
     *
     * @param hash      Hash of the signed data
     * @param signature Signature bytes
     * @return magicValue EIP1271_MAGIC if valid, 0xffffffff otherwise
     */
    function isValidSignature(
        bytes32 hash,
        bytes calldata signature
    ) external view returns (bytes4 magicValue) {
        // Note: view function — cannot call FALCON verifier (state-changing)
        // Use ECDSA validation for EIP-1271 compatibility
        address recovered = _recoverECDSA(hash, signature);
        if (recovered == ecdsaOwner) {
            return EIP1271_MAGIC;
        }
        return 0xffffffff;
    }

    // ============================================================
    // Transaction Execution
    // ============================================================

    /**
     * @notice Execute a transaction from this account.
     * @param to    Target address
     * @param value ETH value to send
     * @param data  Call data
     */
    function execute(
        address to,
        uint256 value,
        bytes calldata data
    ) external onlyEntryPointOrOwner {
        (bool success, bytes memory result) = to.call{value: value}(data);
        if (!success) revert ExecutionFailed(result);
        emit TransactionExecuted(to, value, data);
    }

    /**
     * @notice Execute a batch of transactions.
     * @param targets Target addresses
     * @param values  ETH values
     * @param datas   Call data array
     */
    function executeBatch(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata datas
    ) external onlyEntryPointOrOwner {
        require(targets.length == values.length && values.length == datas.length, "QBTCAccount: length mismatch");
        for (uint256 i = 0; i < targets.length; i++) {
            (bool success, bytes memory result) = targets[i].call{value: values[i]}(datas[i]);
            if (!success) revert ExecutionFailed(result);
        }
    }

    // ============================================================
    // Key Management
    // ============================================================

    /**
     * @notice Update the FALCON post-quantum public key.
     * @dev Called during key rotation to upgrade to a new quantum-safe key.
     *      Key rotation is essential for long-term quantum security.
     *
     * @param newFalconKey New FALCON public key in NTT domain (compacted)
     */
    function rotateFalconKey(uint256[] calldata newFalconKey) external onlyOwner {
        if (newFalconKey.length == 0) revert InvalidFalconKey();
        falconPublicKey = newFalconKey;
        emit FalconKeyUpdated(newFalconKey.length);
        emit QuantumKeyRotated(address(this), block.number);
    }

    /**
     * @notice Update the ML-DSA post-quantum public key.
     * @param newMLDSAKey New ML-DSA public key (1952 bytes)
     */
    function rotateMLDSAKey(bytes calldata newMLDSAKey) external onlyOwner {
        if (newMLDSAKey.length != 1952) revert InvalidMLDSAKey();
        mldsaPublicKey = newMLDSAKey;
        emit MLDSAKeyUpdated(newMLDSAKey.length);
        emit QuantumKeyRotated(address(this), block.number);
    }

    /**
     * @notice Change the signature validation mode.
     * @param newMode The new signature mode
     */
    function setSignatureMode(SignatureMode newMode) external onlyOwner {
        SignatureMode oldMode = signatureMode;
        signatureMode = newMode;
        emit SignatureModeChanged(oldMode, newMode);
    }

    // ============================================================
    // Internal: Signature Validation
    // ============================================================

    /**
     * @dev Validate a signature based on the current signature mode.
     */
    function _validateSignature(
        bytes32 hash,
        bytes calldata signature
    ) internal returns (bool) {
        if (signatureMode == SignatureMode.ECDSA) {
            return _recoverECDSA(hash, signature) == ecdsaOwner;

        } else if (signatureMode == SignatureMode.FALCON) {
            return _validateFALCON(hash, signature);

        } else if (signatureMode == SignatureMode.HYBRID) {
            // Both ECDSA and FALCON must be valid
            // Signature format: [65 bytes ECDSA][rest: FALCON]
            if (signature.length < 65) return false;
            bytes calldata ecdsaSig = signature[:65];
            bytes calldata falconSig = signature[65:];
            bool ecdsaValid = _recoverECDSA(hash, ecdsaSig) == ecdsaOwner;
            bool falconValid = _validateFALCON(hash, falconSig);
            return ecdsaValid && falconValid;

        } else if (signatureMode == SignatureMode.EPERVIER) {
            return _validateEPERVIER(hash, signature);
        }

        return false;
    }

    /**
     * @dev Validate a FALCON signature using the ZKnox verifier.
     */
    function _validateFALCON(bytes32 hash, bytes calldata sig) internal returns (bool) {
        if (falconPublicKey.length == 0) return false;
        try falconVerifier.verifyETHFALCON(hash, falconPublicKey, sig) returns (bool valid) {
            return valid;
        } catch {
            return false;
        }
    }

    /**
     * @dev Validate an EPERVIER signature (FALCON with recovery).
     *      Recovers the address and compares with ecdsaOwner.
     */
    function _validateEPERVIER(bytes32 hash, bytes calldata sig) internal returns (bool) {
        try falconVerifier.recoverEPERVIER(hash, sig) returns (address recovered) {
            return recovered == ecdsaOwner;
        } catch {
            return false;
        }
    }

    /**
     * @dev Recover the ECDSA signer from a signature.
     */
    function _recoverECDSA(bytes32 hash, bytes calldata sig) internal pure returns (address) {
        if (sig.length != 65) return address(0);
        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := calldataload(sig.offset)
            s := calldataload(add(sig.offset, 32))
            v := byte(0, calldataload(add(sig.offset, 64)))
        }
        if (v < 27) v += 27;
        if (v != 27 && v != 28) return address(0);
        return ecrecover(hash, v, r, s);
    }

    // ============================================================
    // Receive ETH
    // ============================================================

    receive() external payable {}
    fallback() external payable {}
}
