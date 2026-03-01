// Package qbp provides the official Go SDK for interacting with a
// Quantum Blockchain Pro (QBP) node.
//
// It allows Go applications to:
//   - Connect to a QBP node via RPC
//   - Manage post-quantum accounts (ML-DSA/ML-KEM)
//   - Send transactions with ML-DSA signatures
//   - Deploy and interact with smart contracts
//   - Submit and retrieve quantum computation jobs
package qbp

import (
	"context"
	"crypto/sha3"
	"fmt"
	"math/big"

	"github.com/NikaHsaini/Quantum-Blockchain-Pro/qbp-chain/crypto/pqcrypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// ============================================================
// QBP Client
// ============================================================

// Client is a QBP client for interacting with a QBP node.
// It embeds an go-ethereum ethclient and adds QBP-specific functionality.
type Client struct {
	*ethclient.Client
	rpc *rpc.Client
}

// Dial connects a client to the given URL.
func Dial(rawurl string) (*Client, error) {
	rpcClient, err := rpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(rpcClient)

	return &Client{
		Client: ethClient,
		rpc:    rpcClient,
	}, nil
}

// ============================================================
// Post-Quantum Account Management
// ============================================================

// Account represents a QBP account with post-quantum keys.
type Account struct {
	Address    common.Address
	PrivateKey *pqcrypto.MLDSAPrivateKey
	PublicKey  *pqcrypto.MLDSAPublicKey
}

// NewAccount creates a new QBP account with a new ML-DSA key pair.
func NewAccount() (*Account, error) {
	keyPair, err := pqcrypto.GenerateMLDSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("qbp-sdk: failed to generate ML-DSA key pair: %w", err)
	}

	addr := keyPair.PublicKey.Address()

	return &Account{
		Address:    common.BytesToAddress(addr[:]),
		PrivateKey: keyPair.PrivateKey,
		PublicKey:  keyPair.PublicKey,
	}, nil
}

// AccountFromPrivateKey creates an Account from an existing ML-DSA private key.
func AccountFromPrivateKey(privKey *pqcrypto.MLDSAPrivateKey) *Account {
	pubKey := privKey.GetPublicKey()
	addr := pubKey.Address()

	return &Account{
		Address:    common.BytesToAddress(addr[:]),
		PrivateKey: privKey,
		PublicKey:  pubKey,
	},
}

// ============================================================
// Transaction Signing
// ============================================================

// SignTx signs a transaction with the account's ML-DSA private key.
// This is the post-quantum equivalent of types.SignTx in go-ethereum.
func SignTx(tx *Transaction, signer Signer, prv *pqcrypto.MLDSAPrivateKey) (*Transaction, error) {
	h := signer.Hash(tx)
	sig, err := pqcrypto.Sign(prv, h[:])
	if err != nil {
		return nil, err
	}

	return tx.WithSignature(signer, sig.Bytes())
}

// Transaction represents a QBP transaction.
// It is compatible with go-ethereum's types.Transaction but uses a different signature scheme.
type Transaction struct {
	// Fields compatible with go-ethereum/core/types.Transaction
	Nonce    uint64
	GasPrice *big.Int
	GasLimit uint64
	To       *common.Address
	Value    *big.Int
	Data     []byte

	// QBP-specific post-quantum signature
	// V, R, S are replaced by a single ML-DSA signature
	PQSignature []byte
}

// WithSignature returns a new transaction with the given signature.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	cpy := *tx
	cpy.PQSignature = sig
	return &cpy, nil
}

// Hash computes the hash of the transaction to be signed.
func (tx *Transaction) Hash() common.Hash {
	h := sha3.New256()
	// Simplified hashing for SDK example
	h.Write(tx.Data)
	return common.BytesToHash(h.Sum(nil))
}

// Signer is an interface for signing transactions.
type Signer interface {
	Hash(*Transaction) common.Hash
}

// ============================================================
// Quantum Oracle Interaction
// ============================================================

// QuantumJob represents a quantum computation job for submission.
type QuantumJob struct {
	NumQubits   uint64
	CircuitHash [32]byte
	Deadline    *big.Int
	Reward      *big.Int
}

// SubmitQuantumJob submits a quantum computation job to the QuantumOracle contract.
func (c *Client) SubmitQuantumJob(ctx context.Context, job *QuantumJob, account *Account) (*Transaction, error) {
	// In a full implementation, this would:
	// 1. ABI-encode the call to QuantumOracle.submitJob()
	// 2. Create a new transaction with the encoded data
	// 3. Sign the transaction with the account's ML-DSA key
	// 4. Send the raw transaction

	fmt.Printf("Submitting quantum job with %d qubits...\n", job.NumQubits)

	// Create a dummy transaction for demonstration
	tx := &Transaction{
		Nonce:    0, // Should be retrieved from the network
		To:       &common.Address{}, // QuantumOracle contract address
		Value:    job.Reward,
		GasLimit: 300000,
		GasPrice: big.NewInt(20000000000), // 20 Gwei
		Data:     []byte("submitJob_encoded_data"),
	}

	// Sign the transaction
	signer := &QBPSigner{}
	signedTx, err := SignTx(tx, signer, account.PrivateKey)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Transaction signed with ML-DSA signature of length %d\n", len(signedTx.PQSignature))

	// Send transaction (simulation)
	// err = c.SendTransaction(ctx, signedTx)
	// if err != nil { ... }

	return signedTx, nil
}

// GetQuantumJobResult retrieves the result of a quantum job.
func (c *Client) GetQuantumJobResult(ctx context.Context, jobID [32]byte) (string, error) {
	var result string
	// In a full implementation, this would call the QuantumOracle.jobs() view function.
	err := c.rpc.CallContext(ctx, &result, "qbp_getQuantumJobResult", jobID)
	if err != nil {
		return "", err
	}
	return result, nil
}

// QBPSigner implements the Signer interface for QBP transactions.
type QBPSigner struct{}

// Hash returns the hash to be signed for a QBP transaction.
func (s *QBPSigner) Hash(tx *Transaction) common.Hash {
	return tx.Hash()
}

// ============================================================
// Example Usage
// ============================================================

// Example demonstrates how to use the QBP Go SDK.
func Example() {
	// 1. Connect to a QBP node
	client, err := Dial("http://localhost:8545")
	if err != nil {
		fmt.Printf("Failed to connect to QBP node: %v\n", err)
		return
	}

	// 2. Create a new post-quantum account
	account, err := NewAccount()
	if err != nil {
		fmt.Printf("Failed to create account: %v\n", err)
		return
	}
	fmt.Printf("New QBP Account: %s\n", account.Address.Hex())

	// 3. Get the latest block number
	block, err := client.BlockByNumber(context.Background(), nil)
	if err != nil {
		fmt.Printf("Failed to get latest block: %v\n", err)
		return
	}
	fmt.Printf("Latest Block: %d\n", block.Number().Uint64())

	// 4. Submit a quantum job
	job := &QuantumJob{
		NumQubits:   8,
		CircuitHash: [32]byte{1, 2, 3},
		Deadline:    big.NewInt(int64(block.NumberU64() + 1000)),
		Reward:      big.NewInt(100 * 1e18), // 100 QBP
	}

	tx, err := client.SubmitQuantumJob(context.Background(), job, account)
	if err != nil {
		fmt.Printf("Failed to submit quantum job: %v\n", err)
		return
	}

	fmt.Printf("Submitted quantum job with transaction hash: %s\n", tx.Hash().Hex())
}
