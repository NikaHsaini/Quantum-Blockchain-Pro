// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title ZKNOX_NTT
 * @author Nika Hsaini — QUBITCOIN Foundation
 * @notice Solidity library implementing the Number Theoretic Transform (NTT)
 *         for use in post-quantum cryptographic verification on the EVM.
 *
 * @dev This library is the Solidity equivalent of the ZKnox NTT repository
 *      (https://github.com/ZKNoxHQ/NTT), adapted for the QUBITCOIN network.
 *
 *      The NTT is the core building block for efficient polynomial multiplication
 *      in FALCON, CRYSTALS-Dilithium, and other lattice-based schemes.
 *
 *      Key parameters:
 *        - Modulus q = 12289 (FALCON prime)
 *        - Degree n = 512 (FALCON-512)
 *        - Primitive root ψ = 7^{(q-1)/(2n)} mod q
 *
 *      The NTT-EIP submitted by ZKnox proposes adding an NTT precompile to
 *      the EVM, which would reduce gas costs by ~10x. This library provides
 *      a pure-Solidity fallback until that precompile is available.
 *
 * Acknowledgements: ZKnox team for the NTT-EIP and ETHFALCON implementations.
 */
library ZKNOX_NTT {

    uint256 constant Q = 12289;
    uint256 constant N = 512;

    /**
     * @notice Compute the forward NTT of a polynomial in-place.
     * @dev Cooley-Tukey butterfly algorithm with precomputed twiddle factors.
     *      Equivalent to ZKNOX_NTTFW in the ZKnox EVM implementation.
     * @param a Input polynomial coefficients (modified in-place)
     * @param psi Twiddle factors in bit-reversed order
     */
    function forward(
        uint256[] memory a,
        uint256[] memory psi
    ) internal pure {
        uint256 n = a.length;
        uint256 k = 1;

        for (uint256 length = n >> 1; length >= 1; length >>= 1) {
            for (uint256 start = 0; start < n; start += 2 * length) {
                uint256 zeta = psi[k++];
                for (uint256 j = start; j < start + length; j++) {
                    uint256 t = mulmod(zeta, a[j + length], Q);
                    a[j + length] = addmod(a[j], Q - t, Q);
                    a[j] = addmod(a[j], t, Q);
                }
            }
        }
    }

    /**
     * @notice Compute the inverse NTT of a polynomial in-place.
     * @dev Gentleman-Sande butterfly algorithm.
     *      Equivalent to ZKNOX_NTTINV in the ZKnox EVM implementation.
     * @param a Input polynomial coefficients (modified in-place)
     * @param psiInv Inverse twiddle factors in bit-reversed order
     * @param nInv Modular inverse of n: n^{-1} mod q
     */
    function inverse(
        uint256[] memory a,
        uint256[] memory psiInv,
        uint256 nInv
    ) internal pure {
        uint256 n = a.length;
        uint256 k = n - 1;

        for (uint256 length = 1; length < n; length <<= 1) {
            for (uint256 start = 0; start < n; start += 2 * length) {
                uint256 zeta = psiInv[k--];
                for (uint256 j = start; j < start + length; j++) {
                    uint256 t = a[j];
                    a[j] = addmod(t, a[j + length], Q);
                    a[j + length] = mulmod(zeta, addmod(t, Q - a[j + length], Q), Q);
                }
            }
        }

        // Normalize by n^{-1}
        for (uint256 i = 0; i < n; i++) {
            a[i] = mulmod(nInv, a[i], Q);
        }
    }

    /**
     * @notice Pointwise multiplication of two polynomials in NTT domain.
     * @dev c[i] = a[i] * b[i] mod q for all i.
     *      This is the core operation for polynomial multiplication via NTT.
     * @param a First polynomial in NTT domain
     * @param b Second polynomial in NTT domain
     * @return c Product polynomial in NTT domain
     */
    function pointwiseMul(
        uint256[] memory a,
        uint256[] memory b
    ) internal pure returns (uint256[] memory c) {
        uint256 n = a.length;
        c = new uint256[](n);
        for (uint256 i = 0; i < n; i++) {
            c[i] = mulmod(a[i], b[i], Q);
        }
    }

    /**
     * @notice Expand a compacted polynomial representation.
     * @dev Compacted format: 16 coefficients of 16 bits per uint256 word.
     *      Equivalent to ZKNOX_NTT_Expand in the ZKnox Solidity implementation.
     * @param compacted Compacted polynomial (array of uint256 words)
     * @param n Polynomial degree
     * @return expanded Expanded polynomial coefficients
     */
    function expand(
        uint256[] memory compacted,
        uint256 n
    ) internal pure returns (uint256[] memory expanded) {
        expanded = new uint256[](n);
        uint256 idx = 0;
        for (uint256 w = 0; w < compacted.length && idx < n; w++) {
            uint256 word = compacted[w];
            for (uint256 b = 0; b < 256 && idx < n; b += 16) {
                expanded[idx++] = (word >> b) & 0xFFFF;
            }
        }
    }

    /**
     * @notice Compact a polynomial to the packed representation.
     * @dev Packs 16 coefficients of 16 bits per uint256 word.
     *      Equivalent to ZKNOX_NTT_Compact in the ZKnox Solidity implementation.
     * @param expanded Expanded polynomial coefficients
     * @return compacted Compacted polynomial
     */
    function compact(
        uint256[] memory expanded
    ) internal pure returns (uint256[] memory compacted) {
        uint256 n = expanded.length;
        uint256 wordsNeeded = (n + 15) / 16;
        compacted = new uint256[](wordsNeeded);
        for (uint256 i = 0; i < n; i++) {
            compacted[i / 16] |= (expanded[i] & 0xFFFF) << ((i % 16) * 16);
        }
    }
}
