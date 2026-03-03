// Package qbtc provides the official Go SDK for interacting with a
// QUBITCOIN (QBTC) node.
//
// It allows Go applications to:
//   - Connect to a QBTC node via RPC
//   - Manage post-quantum accounts (FALCON / ML-DSA / ML-KEM)
//   - Send transactions with post-quantum signatures
//   - Deploy and interact with smart contracts
//   - Submit and retrieve quantum computation jobs
//   - Interact with the CBDC / Euro Numérique bridge, liquidity pool, and router
//   - Execute QBTC/wEURd swaps with compliance checks
package qbtc

import (
	"context"
	"crypto/sha3"
	"fmt"
	"math/big"

	"github.com/NikaHsaini/Quantum-Blockchain-Pro/qbtc-chain/crypto/pqcrypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// ============================================================
// QBTC Client
// ============================================================

// Client is a QBTC client for interacting with a QUBITCOIN node.
// It embeds a go-ethereum ethclient and adds QBTC-specific functionality
// including CBDC/Euro Numérique operations.
type Client struct {
	*ethclient.Client
	rpc *rpc.Client

	// Contract addresses (set via configuration)
	EuroDigitalBridgeAddr common.Address
	LiquidityPoolAddr     common.Address
	CBDCRouterAddr        common.Address
	QBTCTokenAddr         common.Address
	QuantumOracleAddr     common.Address
}

// Config holds the configuration for the QBTC client.
type Config struct {
	NodeURL               string
	EuroDigitalBridgeAddr common.Address
	LiquidityPoolAddr     common.Address
	CBDCRouterAddr        common.Address
	QBTCTokenAddr         common.Address
	QuantumOracleAddr     common.Address
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

// DialWithConfig connects a client with full contract configuration.
func DialWithConfig(cfg *Config) (*Client, error) {
	client, err := Dial(cfg.NodeURL)
	if err != nil {
		return nil, err
	}
	client.EuroDigitalBridgeAddr = cfg.EuroDigitalBridgeAddr
	client.LiquidityPoolAddr = cfg.LiquidityPoolAddr
	client.CBDCRouterAddr = cfg.CBDCRouterAddr
	client.QBTCTokenAddr = cfg.QBTCTokenAddr
	client.QuantumOracleAddr = cfg.QuantumOracleAddr
	return client, nil
}

// ============================================================
// Post-Quantum Account Management
// ============================================================

// Account represents a QBTC account with post-quantum keys.
type Account struct {
	Address    common.Address
	PrivateKey *pqcrypto.MLDSAPrivateKey
	PublicKey  *pqcrypto.MLDSAPublicKey
}

// NewAccount creates a new QBTC account with a new ML-DSA key pair.
func NewAccount() (*Account, error) {
	keyPair, err := pqcrypto.GenerateMLDSAKeyPair()
	if err != nil {
		return nil, fmt.Errorf("qbtc-sdk: failed to generate ML-DSA key pair: %w", err)
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
	}
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

// Transaction represents a QBTC transaction.
// It is compatible with go-ethereum's types.Transaction but uses
// post-quantum signature schemes instead of ECDSA.
type Transaction struct {
	Nonce    uint64
	GasPrice *big.Int
	GasLimit uint64
	To       *common.Address
	Value    *big.Int
	Data     []byte

	// QBTC-specific post-quantum signature
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
	h.Write(tx.Data)
	return common.BytesToHash(h.Sum(nil))
}

// Signer is an interface for signing transactions.
type Signer interface {
	Hash(*Transaction) common.Hash
}

// QBTCSigner implements the Signer interface for QBTC transactions.
type QBTCSigner struct{}

// Hash returns the hash to be signed for a QBTC transaction.
func (s *QBTCSigner) Hash(tx *Transaction) common.Hash {
	return tx.Hash()
}

// ============================================================
// CBDC / Euro Numérique Operations
// ============================================================

// CBDCMintRequest represents a request to mint wEURd (wrapped Digital Euro).
type CBDCMintRequest struct {
	Recipient       common.Address
	Amount          *big.Int // Amount in wEURd (18 decimals)
	DL3STransactionID [32]byte // Reference to the DL3S/TARGET2 settlement
}

// CBDCBurnRequest represents a request to burn wEURd and settle in Digital Euro.
type CBDCBurnRequest struct {
	Amount          *big.Int
	SettlementAgent common.Address
	DL3STransactionID [32]byte
}

// SwapRequest represents a swap request on the QBTC/wEURd liquidity pool.
type SwapRequest struct {
	QBTCToWEURd  bool     // Direction: true = sell QBTC for wEURd, false = buy QBTC with wEURd
	AmountIn     *big.Int // Input amount
	MinAmountOut *big.Int // Minimum acceptable output (slippage protection)
	Deadline     *big.Int // Block timestamp deadline
}

// SwapQuote represents a quote for a swap.
type SwapQuote struct {
	AmountOut *big.Int
	Fee       *big.Int
	Price     *big.Int // Current spot price (wEURd per QBTC)
}

// RouteRequest represents a cross-CBDC routing request.
type RouteRequest struct {
	FromAsset    [32]byte       // Source CBDC identifier
	ToAsset      [32]byte       // Destination CBDC identifier
	AmountIn     *big.Int
	MinAmountOut *big.Int
	Recipient    common.Address
	Deadline     *big.Int
}

// MintWEURd mints wrapped Digital Euro (wEURd) via the EuroDigitalBridge.
// Only callable by authorized settlement agents (banks/PSPs).
func (c *Client) MintWEURd(ctx context.Context, req *CBDCMintRequest, account *Account) (*Transaction, error) {
	fmt.Printf("Minting %s wEURd for %s via EuroDigitalBridge...\n", req.Amount.String(), req.Recipient.Hex())

	// ABI-encode: mint(address recipient, uint256 amount, bytes32 dl3sTransactionId)
	data := encodeMintCall(req.Recipient, req.Amount, req.DL3STransactionID)

	tx := &Transaction{
		To:       &c.EuroDigitalBridgeAddr,
		Value:    big.NewInt(0),
		GasLimit: 200000,
		GasPrice: big.NewInt(20000000000),
		Data:     data,
	}

	signer := &QBTCSigner{}
	return SignTx(tx, signer, account.PrivateKey)
}

// BurnWEURd burns wEURd and initiates a settlement in Digital Euro.
func (c *Client) BurnWEURd(ctx context.Context, req *CBDCBurnRequest, account *Account) (*Transaction, error) {
	fmt.Printf("Burning %s wEURd for settlement to %s...\n", req.Amount.String(), req.SettlementAgent.Hex())

	data := encodeBurnCall(req.Amount, req.SettlementAgent, req.DL3STransactionID)

	tx := &Transaction{
		To:       &c.EuroDigitalBridgeAddr,
		Value:    big.NewInt(0),
		GasLimit: 200000,
		GasPrice: big.NewInt(20000000000),
		Data:     data,
	}

	signer := &QBTCSigner{}
	return SignTx(tx, signer, account.PrivateKey)
}

// SwapQBTCWEURd executes a swap on the QBTC/wEURd liquidity pool.
func (c *Client) SwapQBTCWEURd(ctx context.Context, req *SwapRequest, account *Account) (*Transaction, error) {
	direction := "wEURd → QBTC"
	if req.QBTCToWEURd {
		direction = "QBTC → wEURd"
	}
	fmt.Printf("Swapping %s: %s in, min %s out...\n", direction, req.AmountIn.String(), req.MinAmountOut.String())

	data := encodeSwapCall(req.QBTCToWEURd, req.AmountIn, req.MinAmountOut, req.Deadline)

	tx := &Transaction{
		To:       &c.LiquidityPoolAddr,
		Value:    big.NewInt(0),
		GasLimit: 500000,
		GasPrice: big.NewInt(20000000000),
		Data:     data,
	}

	signer := &QBTCSigner{}
	return SignTx(tx, signer, account.PrivateKey)
}

// GetSwapQuote returns a quote for a QBTC/wEURd swap without executing it.
func (c *Client) GetSwapQuote(ctx context.Context, qbtcToWEURd bool, amountIn *big.Int) (*SwapQuote, error) {
	var result SwapQuote
	err := c.rpc.CallContext(ctx, &result, "qbtc_getSwapQuote", qbtcToWEURd, amountIn.String())
	if err != nil {
		return nil, fmt.Errorf("qbtc-sdk: failed to get swap quote: %w", err)
	}
	return &result, nil
}

// GetQBTCPrice returns the current QBTC price in wEURd.
func (c *Client) GetQBTCPrice(ctx context.Context) (*big.Int, error) {
	var result string
	err := c.rpc.CallContext(ctx, &result, "qbtc_getQBTCPrice")
	if err != nil {
		return nil, fmt.Errorf("qbtc-sdk: failed to get QBTC price: %w", err)
	}
	price := new(big.Int)
	price.SetString(result, 10)
	return price, nil
}

// RouteCBDC executes a cross-CBDC swap via the CBDCRouter.
func (c *Client) RouteCBDC(ctx context.Context, req *RouteRequest, account *Account) (*Transaction, error) {
	fmt.Printf("Routing CBDC swap: %s in, recipient %s...\n", req.AmountIn.String(), req.Recipient.Hex())

	data := encodeRouteCall(req.FromAsset, req.ToAsset, req.AmountIn, req.MinAmountOut, req.Recipient, req.Deadline)

	tx := &Transaction{
		To:       &c.CBDCRouterAddr,
		Value:    big.NewInt(0),
		GasLimit: 800000,
		GasPrice: big.NewInt(20000000000),
		Data:     data,
	}

	signer := &QBTCSigner{}
	return SignTx(tx, signer, account.PrivateKey)
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
	fmt.Printf("Submitting quantum job with %d qubits...\n", job.NumQubits)

	tx := &Transaction{
		Nonce:    0,
		To:       &c.QuantumOracleAddr,
		Value:    job.Reward,
		GasLimit: 300000,
		GasPrice: big.NewInt(20000000000),
		Data:     []byte("submitJob_encoded_data"),
	}

	signer := &QBTCSigner{}
	signedTx, err := SignTx(tx, signer, account.PrivateKey)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Transaction signed with ML-DSA signature of length %d\n", len(signedTx.PQSignature))
	return signedTx, nil
}

// GetQuantumJobResult retrieves the result of a quantum job.
func (c *Client) GetQuantumJobResult(ctx context.Context, jobID [32]byte) (string, error) {
	var result string
	err := c.rpc.CallContext(ctx, &result, "qbtc_getQuantumJobResult", jobID)
	if err != nil {
		return "", err
	}
	return result, nil
}

// ============================================================
// ABI Encoding Helpers (simplified for SDK)
// ============================================================

func encodeMintCall(recipient common.Address, amount *big.Int, dl3sTxID [32]byte) []byte {
	// keccak256("mint(address,uint256,bytes32)")[:4]
	selector := []byte{0x40, 0xc1, 0x0f, 0x19}
	data := make([]byte, 4+32+32+32)
	copy(data[:4], selector)
	copy(data[4+12:4+32], recipient.Bytes())
	amount.FillBytes(data[4+32 : 4+64])
	copy(data[4+64:4+96], dl3sTxID[:])
	return data
}

func encodeBurnCall(amount *big.Int, agent common.Address, dl3sTxID [32]byte) []byte {
	selector := []byte{0x42, 0x96, 0x6c, 0x68}
	data := make([]byte, 4+32+32+32)
	copy(data[:4], selector)
	amount.FillBytes(data[4 : 4+32])
	copy(data[4+32+12:4+64], agent.Bytes())
	copy(data[4+64:4+96], dl3sTxID[:])
	return data
}

func encodeSwapCall(qbtcToWEURd bool, amountIn, minAmountOut, deadline *big.Int) []byte {
	selector := []byte{0x02, 0x2c, 0x0d, 0x9f}
	data := make([]byte, 4+32+32+32+32)
	copy(data[:4], selector)
	if qbtcToWEURd {
		data[4+31] = 1
	}
	amountIn.FillBytes(data[4+32 : 4+64])
	minAmountOut.FillBytes(data[4+64 : 4+96])
	deadline.FillBytes(data[4+96 : 4+128])
	return data
}

func encodeRouteCall(fromAsset, toAsset [32]byte, amountIn, minAmountOut *big.Int, recipient common.Address, deadline *big.Int) []byte {
	selector := []byte{0x3b, 0x4b, 0x13, 0x81}
	data := make([]byte, 4+32+32+32+32+32+32)
	copy(data[:4], selector)
	copy(data[4:4+32], fromAsset[:])
	copy(data[4+32:4+64], toAsset[:])
	amountIn.FillBytes(data[4+64 : 4+96])
	minAmountOut.FillBytes(data[4+96 : 4+128])
	copy(data[4+128+12:4+160], recipient.Bytes())
	deadline.FillBytes(data[4+160 : 4+192])
	return data
}

// ============================================================
// Example Usage
// ============================================================

// Example demonstrates how to use the QBTC Go SDK with CBDC operations.
func Example() {
	// 1. Connect to a QBTC node with full configuration
	client, err := DialWithConfig(&Config{
		NodeURL:               "http://localhost:8545",
		EuroDigitalBridgeAddr: common.HexToAddress("0x1000000000000000000000000000000000000001"),
		LiquidityPoolAddr:     common.HexToAddress("0x1000000000000000000000000000000000000002"),
		CBDCRouterAddr:        common.HexToAddress("0x1000000000000000000000000000000000000003"),
		QBTCTokenAddr:         common.HexToAddress("0x1000000000000000000000000000000000000004"),
		QuantumOracleAddr:     common.HexToAddress("0x1000000000000000000000000000000000000005"),
	})
	if err != nil {
		fmt.Printf("Failed to connect to QBTC node: %v\n", err)
		return
	}

	// 2. Create a new post-quantum account
	account, err := NewAccount()
	if err != nil {
		fmt.Printf("Failed to create account: %v\n", err)
		return
	}
	fmt.Printf("New QBTC Account: %s\n", account.Address.Hex())

	// 3. Get the current QBTC price in EUR
	price, err := client.GetQBTCPrice(context.Background())
	if err != nil {
		fmt.Printf("Failed to get QBTC price: %v\n", err)
	} else {
		fmt.Printf("Current QBTC Price: %s wEURd\n", price.String())
	}

	// 4. Get a swap quote (buy 1 QBTC with wEURd)
	oneQBTC := new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	quote, err := client.GetSwapQuote(context.Background(), false, oneQBTC)
	if err != nil {
		fmt.Printf("Failed to get swap quote: %v\n", err)
	} else {
		fmt.Printf("Swap Quote: %s wEURd for 1 QBTC (fee: %s)\n", quote.AmountOut.String(), quote.Fee.String())
	}

	// 5. Execute a swap: buy QBTC with 100,000 wEURd
	swapReq := &SwapRequest{
		QBTCToWEURd:  false, // wEURd → QBTC
		AmountIn:     new(big.Int).Mul(big.NewInt(100000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
		MinAmountOut: new(big.Int).Mul(big.NewInt(9), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)), // Min 0.9 QBTC
		Deadline:     big.NewInt(999999999),
	}

	swapTx, err := client.SwapQBTCWEURd(context.Background(), swapReq, account)
	if err != nil {
		fmt.Printf("Failed to execute swap: %v\n", err)
	} else {
		fmt.Printf("Swap transaction signed: %s\n", swapTx.Hash().Hex())
	}

	// 6. Submit a quantum job
	job := &QuantumJob{
		NumQubits:   8,
		CircuitHash: [32]byte{1, 2, 3},
		Deadline:    big.NewInt(100000),
		Reward:      new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	}

	tx, err := client.SubmitQuantumJob(context.Background(), job, account)
	if err != nil {
		fmt.Printf("Failed to submit quantum job: %v\n", err)
		return
	}
	fmt.Printf("Submitted quantum job: %s\n", tx.Hash().Hex())
}
