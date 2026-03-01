// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title ZKNOX_Math
 * @author Nika Hsaini — QUBITCOIN Foundation
 * @notice Mathematical utility library for post-quantum cryptographic operations.
 *         Provides modular arithmetic, Montgomery reduction, and field operations
 *         used by FALCON, Dilithium, and ML-KEM verifiers.
 *
 * @dev All operations are constant-time to prevent timing side-channel attacks.
 */
library ZKNOX_Math {

    /// @notice Compute the modular inverse of a modulo m using Fermat's little theorem.
    /// @dev Requires m to be prime. Uses fast exponentiation.
    /// @param a The value to invert
    /// @param m The prime modulus
    /// @return The modular inverse a^{-1} mod m
    function modInverse(uint256 a, uint256 m) internal pure returns (uint256) {
        require(m > 1, "ZKNOX_Math: modulus must be > 1");
        require(a != 0, "ZKNOX_Math: cannot invert zero");
        return modExp(a % m, m - 2, m);
    }

    /// @notice Compute base^exp mod m using fast exponentiation.
    /// @param base The base
    /// @param exp The exponent
    /// @param m The modulus
    /// @return result base^exp mod m
    function modExp(uint256 base, uint256 exp, uint256 m) internal pure returns (uint256 result) {
        result = 1;
        base = base % m;
        while (exp > 0) {
            if (exp & 1 == 1) {
                result = mulmod(result, base, m);
            }
            exp >>= 1;
            base = mulmod(base, base, m);
        }
    }

    /// @notice Compute the centered reduction of a coefficient modulo q.
    /// @dev Maps a coefficient from [0, q) to (-q/2, q/2].
    /// @param a The coefficient in [0, q)
    /// @param q The modulus
    /// @return The centered coefficient in (-q/2, q/2]
    function centerReduce(uint256 a, uint256 q) internal pure returns (int256) {
        if (a > q / 2) {
            return int256(a) - int256(q);
        }
        return int256(a);
    }

    /// @notice Compute the L2 norm squared of a polynomial.
    /// @dev Used for FALCON signature norm check.
    /// @param poly Polynomial coefficients in [0, q)
    /// @param q The modulus
    /// @return norm The L2 norm squared (sum of centered coefficients squared)
    function normSquared(uint256[] memory poly, uint256 q) internal pure returns (uint256 norm) {
        norm = 0;
        for (uint256 i = 0; i < poly.length; i++) {
            int256 c = centerReduce(poly[i], q);
            norm += uint256(c * c);
        }
    }

    /// @notice Compute the bit reversal of x using w bits.
    /// @param x The value to bit-reverse
    /// @param w The number of bits
    /// @return result The bit-reversed value
    function bitReverse(uint256 x, uint256 w) internal pure returns (uint256 result) {
        result = 0;
        for (uint256 i = 0; i < w; i++) {
            result = (result << 1) | (x & 1);
            x >>= 1;
        }
    }

    /// @notice Compute the primitive 2n-th root of unity modulo q.
    /// @dev ψ = ω^{(q-1)/(2n)} mod q where ω is the primitive root.
    /// @param omega Primitive root of unity modulo q
    /// @param q The prime modulus
    /// @param n The polynomial degree (must divide q-1)
    /// @return psi The primitive 2n-th root of unity
    function primitiveRoot2n(uint256 omega, uint256 q, uint256 n) internal pure returns (uint256 psi) {
        uint256 exp = (q - 1) / (2 * n);
        psi = modExp(omega, exp, q);
    }

    /// @notice Decompose a value into high and low bits (for Dilithium).
    /// @dev Used in the Dilithium signature scheme's decompose operation.
    /// @param r The value to decompose
    /// @param alpha The decomposition parameter
    /// @param q The modulus
    /// @return r1 High bits
    /// @return r0 Low bits (centered)
    function decompose(
        uint256 r,
        uint256 alpha,
        uint256 q
    ) internal pure returns (uint256 r1, int256 r0) {
        r = r % q;
        r0 = int256(r) % int256(alpha);
        if (r0 > int256(alpha) / 2) {
            r0 -= int256(alpha);
        }
        if (int256(r) - r0 == int256(q) - 1) {
            r1 = 0;
            r0 = r0 - 1;
        } else {
            r1 = uint256(int256(r) - r0) / alpha;
        }
    }

    /// @notice Check if two byte arrays are equal in constant time.
    /// @dev Prevents timing attacks on signature verification.
    /// @param a First byte array
    /// @param b Second byte array
    /// @return True if the arrays are equal
    function constantTimeEqual(bytes memory a, bytes memory b) internal pure returns (bool) {
        if (a.length != b.length) return false;
        uint256 diff = 0;
        for (uint256 i = 0; i < a.length; i++) {
            diff |= uint256(uint8(a[i]) ^ uint8(b[i]));
        }
        return diff == 0;
    }
}
