// qbp is the main command-line interface for the Quantum Blockchain Pro node.
//
// It provides a full suite of commands to run a QBP node, manage accounts,
// interact with the blockchain, and access quantum computation features.
//
// Examples:
//   - qbp node start --network mainnet --unlock "0x..."
//   - qbp account new --pq-secure
//   - qbp console attach
//   - qbp quantum submit-job --circuit circuit.json --reward 100
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/urfave/cli/v2"
)

var (
	// app is the main CLI application instance.
	app = cli.NewApp()
)

func init() {
	// Initialize the CLI application metadata
	app.Name = "qbp"
	app.Version = "1.0.0-alpha"
	app.Usage = "Quantum Blockchain Pro Command-Line Interface"
	app.Copyright = "Copyright 2026 Nika Hsaini & Manus AI"
	app.Authors = []*cli.Author{
		{
			Name:  "Nika Hsaini",
			Email: "nika.hsaini@example.com",
		},
		{
			Name:  "Manus AI",
			Email: "contact@manus.im",
		},
	}

	// Define the set of commands available in the CLI
	app.Commands = []*cli.Command{
		// Node management commands
		{
			Name:        "node",
			Usage:       "Manage the QBP node",
			Subcommands: []*cli.Command{
				{
					Name:   "start",
					Usage:  "Start the QBP node and connect to the network",
					Action: startNode,
					Flags:  nodeFlags,
				},
				{
					Name:   "status",
					Usage:  "Get the status of the running node",
					Action: nodeStatus,
				},
			},
		},
		// Account management commands
		{
			Name:        "account",
			Usage:       "Manage QBP accounts",
			Subcommands: []*cli.Command{
				{
					Name:   "new",
					Usage:  "Create a new QBP account with a post-quantum key pair",
					Action: newAccount,
					Flags:  []cli.Flag{
						&cli.BoolFlag{
							Name:  "pq-secure",
							Usage: "Enable post-quantum security for the account",
						},
					},
				},
				{
					Name:   "list",
					Usage:  "List all accounts in the keystore",
					Action: listAccounts,
				},
				{
					Name:   "import",
					Usage:  "Import a private key into a new account",
					Action: importAccount,
				},
			},
		},
		// Console and attachment commands
		{
			Name:   "console",
			Usage:  "Attach an interactive JavaScript console to a running node",
			Action: attachConsole,
		},
		{
			Name:   "attach",
			Usage:  "Attach to a running node (alias for console)",
			Action: attachConsole,
		},
		// Quantum computation commands
		{
			Name:        "quantum",
			Usage:       "Interact with the Quantum as a Service (QMaaS) marketplace",
			Subcommands: []*cli.Command{
				{
					Name:   "submit-job",
					Usage:  "Submit a quantum computation job",
					Action: submitQuantumJob,
					Flags:  []cli.Flag{
						&cli.StringFlag{
							Name:     "circuit",
							Usage:    "Path to the quantum circuit JSON file",
							Required: true,
						},
						&cli.Float64Flag{
							Name:     "reward",
							Usage:    "Reward in QBP for the miner who solves the job",
							Required: true,
						},
					},
				},
				{
					Name:   "get-result",
					Usage:  "Get the result of a completed quantum job",
					Action: getQuantumResult,
				},
			},
		},
		// Blockchain interaction commands
		{
			Name:        "chain",
			Usage:       "Interact with the blockchain",
			Subcommands: []*cli.Command{
				{
					Name:   "get-block",
					Usage:  "Get a block by number or hash",
					Action: getBlock,
				},
				{
					Name:   "get-tx",
					Usage:  "Get a transaction by hash",
					Action: getTransaction,
				},
			},
		},
	}

	// Sort commands for consistent help output
	sort.Sort(cli.CommandsByName(app.Commands))
}

// main is the entry point of the qbp application.
func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ============================================================
// Command Implementations (Stubs)
// ============================================================

// startNode is the action for the "node start" command.
func startNode(ctx *cli.Context) error {
	fmt.Println("Starting QBP node...")
	// In a full implementation, this would:
	// 1. Parse flags (network, datadir, port, etc.)
	// 2. Create a Node instance from go-ethereum/node
	// 3. Configure the QPoA consensus engine
	// 4. Register the Quantum EVM and QMaaS engine
	// 5. Start the P2P server
	// 6. Start the JSON-RPC API
	// 7. Wait for the node to stop
	fmt.Println("Network:", ctx.String("network"))
	fmt.Println("Data Directory:", ctx.String("datadir"))
	fmt.Println("Node started successfully. (Simulation)")
	return nil
}

// nodeStatus is the action for the "node status" command.
func nodeStatus(ctx *cli.Context) error {
	fmt.Println("Getting node status...")
	// In a full implementation, this would connect to the running node's
	// IPC or RPC endpoint and query its status (syncing, block number, peers).
	fmt.Println("Node is running. (Simulation)")
	fmt.Println("Current Block: 12345")
	fmt.Println("Peers: 5")
	return nil
}

// newAccount is the action for the "account new" command.
func newAccount(ctx *cli.Context) error {
	fmt.Println("Creating new QBP account...")
	// In a full implementation, this would:
	// 1. Call pqcrypto.NewQBPAccount() to generate ML-DSA and ML-KEM keys
	// 2. Prompt for a password to encrypt the private key
	// 3. Save the encrypted key to the keystore directory
	// 4. If --pq-secure is passed, register the ML-DSA key on-chain
	fmt.Println("Account created successfully. (Simulation)")
	fmt.Println("Address: 0xAbc...Def")
	fmt.Println("Saved to keystore.")
	return nil
}

// listAccounts is the action for the "account list" command.
func listAccounts(ctx *cli.Context) error {
	fmt.Println("Listing QBP accounts...")
	// In a full implementation, this would scan the keystore directory
	// and list all found accounts.
	fmt.Println("Account #0: {123...456} 0xAbc...Def")
	fmt.Println("Account #1: {789...012} 0xGhi...Jkl")
	return nil
}

// importAccount is the action for the "account import" command.
func importAccount(ctx *cli.Context) error {
	fmt.Println("Importing private key...")
	// In a full implementation, this would:
	// 1. Prompt for the private key (hex encoded)
	// 2. Prompt for a password
	// 3. Encrypt and save to keystore
	fmt.Println("Account imported successfully. (Simulation)")
	return nil
}

// attachConsole is the action for the "console" and "attach" commands.
func attachConsole(ctx *cli.Context) error {
	fmt.Println("Attaching to QBP node...")
	// In a full implementation, this would connect to the node's IPC endpoint
	// and start an interactive JavaScript console.
	fmt.Println("Welcome to the Quantum Blockchain Pro JavaScript Console!")
	fmt.Println("> qbp.blockNumber")
	fmt.Println("12345")
	return nil
}

// submitQuantumJob is the action for the "quantum submit-job" command.
func submitQuantumJob(ctx *cli.Context) error {
	fmt.Println("Submitting quantum job...")
	// In a full implementation, this would:
	// 1. Read the circuit file
	// 2. Connect to the QBP node
	// 3. Call the QuantumOracle.submitJob() smart contract function
	fmt.Println("Circuit File:", ctx.String("circuit"))
	fmt.Println("Reward:", ctx.Float64("reward"), "QBP")
	fmt.Println("Job submitted successfully. Job ID: 0x123...abc")
	return nil
}

// getQuantumResult is the action for the "quantum get-result" command.
func getQuantumResult(ctx *cli.Context) error {
	fmt.Println("Getting quantum job result...")
	// In a full implementation, this would call the QuantumOracle.jobs() function
	// to get the result hash and other details.
	fmt.Println("Job ID:", ctx.Args().First())
	fmt.Println("Status: Completed")
	fmt.Println("Result Hash: 0xdef...456")
	return nil
}

// getBlock is the action for the "chain get-block" command.
func getBlock(ctx *cli.Context) error {
	fmt.Println("Getting block...")
	// In a full implementation, this would call the qbp.getBlock RPC method.
	fmt.Println("Block Number:", ctx.Args().First())
	fmt.Println("Hash: 0xabc...123")
	return nil
}

// getTransaction is the action for the "chain get-tx" command.
func getTransaction(ctx *cli.Context) error {
	fmt.Println("Getting transaction...")
	// In a full implementation, this would call the qbp.getTransactionByHash RPC method.
	fmt.Println("Transaction Hash:", ctx.Args().First())
	fmt.Println("From: 0xAbc...Def")
	fmt.Println("To: 0xGhi...Jkl")
	return nil
}

// nodeFlags defines the CLI flags for the "node start" command.
var nodeFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "network",
		Value: "mainnet",
		Usage: "QBP network to connect to (mainnet, testnet)",
	},
	&cli.StringFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
	},
	&cli.IntFlag{
		Name:  "port",
		Value: 30303,
		Usage: "Network listening port",
	},
	&cli.StringFlag{
		Name:  "unlock",
		Usage: "Comma-separated list of accounts to unlock",
	},
	&cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-interactive unlocking",
	},
	&cli.BoolFlag{
		Name:  "mine",
		Usage: "Enable quantum mining (providing computation)",
	},
	&cli.IntFlag{
		Name:  "miner.threads",
		Value: 1,
		Usage: "Number of CPU threads to use for quantum mining",
	},
}
