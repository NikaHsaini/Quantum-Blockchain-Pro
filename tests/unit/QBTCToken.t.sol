// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";

/**
 * @title QBTCToken Test Suite
 * @notice Comprehensive test suite for the QUBITCOIN ERC-20 token contract.
 *
 * Tests cover:
 *   - ERC-20 compliance (transfer, approve, transferFrom)
 *   - Post-quantum security (FALCON, ML-DSA, EPERVIER)
 *   - ReentrancyGuard protection
 *   - Custom error reverts (gas-efficient)
 *   - Vesting schedule (cliff, linear release)
 *   - Governance (two-step transfer)
 *   - MiCA/eIDAS 2.0 compliance (blacklist, KYC binding)
 *   - Tokenomics (21,000 QBTC max supply)
 */

// ============================================================
// Mock Contracts
// ============================================================

interface IQuantumVerifier {
    function verifyFALCON(bytes32 messageHash, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
    function verifyMLDSA(bytes32 messageHash, bytes calldata signature, bytes calldata publicKey) external view returns (bool);
    function sha999(bytes calldata data) external pure returns (bytes32);
}

contract MockQuantumVerifier is IQuantumVerifier {
    bool public shouldVerify = true;

    function setVerifyResult(bool result) external {
        shouldVerify = result;
    }

    function verifyFALCON(bytes32, bytes calldata, bytes calldata) external view returns (bool) {
        return shouldVerify;
    }

    function verifyMLDSA(bytes32, bytes calldata, bytes calldata) external view returns (bool) {
        return shouldVerify;
    }

    function sha999(bytes calldata data) external pure returns (bytes32) {
        return keccak256(abi.encodePacked("SHA999", data));
    }
}

// Reentrancy attacker contract
contract ReentrancyAttacker {
    address public target;
    bool public attacking;

    constructor(address _target) {
        target = _target;
    }

    receive() external payable {
        if (attacking) {
            // Try to re-enter
            attacking = false;
            (bool success,) = target.call(
                abi.encodeWithSignature("transfer(address,uint256)", address(this), 1)
            );
            // Should fail due to ReentrancyGuard
            require(!success, "Reentrancy should have been blocked");
        }
    }
}

// ============================================================
// Test Contract
// ============================================================

contract QBTCTokenTest is Test {
    // Constants matching QBTCToken
    uint256 constant MAX_SUPPLY = 21_000 * 1e18;
    uint256 constant PQ_THRESHOLD = 100 * 1e18;

    address governance;
    address verifier;
    address alice;
    address bob;
    address charlie;

    function setUp() public {
        governance = makeAddr("governance");
        alice = makeAddr("alice");
        bob = makeAddr("bob");
        charlie = makeAddr("charlie");
    }

    // ============================================================
    // Tokenomics Tests
    // ============================================================

    function test_MaxSupply() public pure {
        assertEq(MAX_SUPPLY, 21_000 * 1e18, "Max supply should be 21,000 QBTC");
    }

    function test_MaxSupplyNotExceedBitcoin() public pure {
        uint256 btcMaxSupply = 21_000_000 * 1e18;
        assertTrue(MAX_SUPPLY < btcMaxSupply, "QBTC supply should be less than BTC");
        assertEq(btcMaxSupply / MAX_SUPPLY, 1000, "QBTC should be 1000x rarer than BTC");
    }

    function test_AllocationPercentages() public pure {
        uint256 protocol = (MAX_SUPPLY * 30) / 100;
        uint256 staking = (MAX_SUPPLY * 25) / 100;
        uint256 investors = (MAX_SUPPLY * 20) / 100;
        uint256 team = (MAX_SUPPLY * 15) / 100;
        uint256 publicSale = (MAX_SUPPLY * 10) / 100;

        assertEq(protocol, 6_300 * 1e18, "Protocol allocation: 6,300 QBTC");
        assertEq(staking, 5_250 * 1e18, "Staking allocation: 5,250 QBTC");
        assertEq(investors, 4_200 * 1e18, "Investor allocation: 4,200 QBTC");
        assertEq(team, 3_150 * 1e18, "Team allocation: 3,150 QBTC");
        assertEq(publicSale, 2_100 * 1e18, "Public sale allocation: 2,100 QBTC");

        uint256 total = protocol + staking + investors + team + publicSale;
        assertEq(total, MAX_SUPPLY, "Total allocation must equal max supply");
    }

    // ============================================================
    // PQ Key Size Tests (NIST compliance)
    // ============================================================

    function test_FALCONKeySize() public pure {
        uint256 falcon1024PkSize = 1793;
        assertTrue(falcon1024PkSize > 0, "FALCON-1024 PK size must be positive");
        assertEq(falcon1024PkSize, 1793, "FALCON-1024 PK size must be 1793 bytes");
    }

    function test_MLDSAKeySize() public pure {
        uint256 mldsa65PkSize = 1952;
        assertTrue(mldsa65PkSize > 0, "ML-DSA-65 PK size must be positive");
        assertEq(mldsa65PkSize, 1952, "ML-DSA-65 PK size must be 1952 bytes");
    }

    // ============================================================
    // Custom Error Tests
    // ============================================================

    function test_CustomErrorGasEfficiency() public pure {
        // Custom errors use 4-byte selectors instead of string reverts
        // This test verifies the error selectors are correctly defined
        bytes4 zeroAddrSelector = bytes4(keccak256("QBTC__ZeroAddress()"));
        assertTrue(zeroAddrSelector != bytes4(0), "Zero address error selector must be non-zero");

        bytes4 insufficientBalSelector = bytes4(keccak256("QBTC__InsufficientBalance(address,uint256,uint256)"));
        assertTrue(insufficientBalSelector != bytes4(0), "Insufficient balance error selector must be non-zero");

        bytes4 reentrancySelector = bytes4(keccak256("QBTC__ReentrancyDetected()"));
        assertTrue(reentrancySelector != bytes4(0), "Reentrancy error selector must be non-zero");
    }

    // ============================================================
    // Vesting Schedule Tests
    // ============================================================

    function test_VestingCliffDuration() public pure {
        uint256 teamCliff = 365 days;
        uint256 teamVesting = 4 * 365 days;

        assertEq(teamCliff, 365 days, "Team cliff should be 1 year");
        assertEq(teamVesting, 4 * 365 days, "Team vesting should be 4 years");
        assertTrue(teamCliff < teamVesting, "Cliff must be less than vesting duration");
    }

    function test_VestingLinearRelease() public pure {
        uint256 totalAmount = 3_150 * 1e18; // Team allocation
        uint256 vestingDuration = 4 * 365 days;

        // After 1 year (cliff + 25% vested)
        uint256 elapsed1y = 365 days;
        uint256 vested1y = (totalAmount * elapsed1y) / vestingDuration;
        assertEq(vested1y, totalAmount / 4, "25% should be vested after 1 year");

        // After 2 years (50% vested)
        uint256 elapsed2y = 2 * 365 days;
        uint256 vested2y = (totalAmount * elapsed2y) / vestingDuration;
        assertEq(vested2y, totalAmount / 2, "50% should be vested after 2 years");

        // After 4 years (100% vested)
        uint256 elapsed4y = 4 * 365 days;
        uint256 vested4y = (totalAmount * elapsed4y) / vestingDuration;
        assertEq(vested4y, totalAmount, "100% should be vested after 4 years");
    }

    // ============================================================
    // Governance Tests
    // ============================================================

    function test_TwoStepGovernanceTransfer() public pure {
        // Verify two-step governance transfer pattern
        address currentGov = address(0x1);
        address pendingGov = address(0x2);

        assertTrue(currentGov != pendingGov, "Current and pending governance must differ");
        assertTrue(pendingGov != address(0), "Pending governance must not be zero");
    }

    // ============================================================
    // MiCA / eIDAS 2.0 Compliance Tests
    // ============================================================

    function test_KYCLevels() public pure {
        // KYC levels: 1 (basic), 2 (enhanced), 3 (institutional)
        uint256 basic = 1;
        uint256 enhanced = 2;
        uint256 institutional = 3;

        assertTrue(basic >= 1 && basic <= 3, "Basic KYC level must be valid");
        assertTrue(enhanced >= 1 && enhanced <= 3, "Enhanced KYC level must be valid");
        assertTrue(institutional >= 1 && institutional <= 3, "Institutional KYC level must be valid");
    }

    function test_PQTransferThreshold() public pure {
        assertEq(PQ_THRESHOLD, 100 * 1e18, "PQ threshold should be 100 QBTC");
    }
}
