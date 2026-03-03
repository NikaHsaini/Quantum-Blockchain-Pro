.PHONY: build test test-go test-solidity clean docker lint security

# Build the QUBITCOIN node
build:
@echo "Building QUBITCOIN node..."
cd qbtc-chain && CGO_ENABLED=1 go build -o ../build/bin/qbtc ./cmd/qbtc

# Run all tests
test: test-go test-solidity

# Run Go unit and integration tests
test-go:
@echo "Running Go tests..."
cd qbtc-chain && go test -v -race -count=1 ./...
cd tests && go test -v -race -count=1 ./...

# Run Solidity tests with Foundry
test-solidity:
@echo "Running Solidity tests..."
forge test -vvv --gas-report

# Run security analysis
security:
@echo "Running Go security checks..."
cd qbtc-chain && govulncheck ./...
cd qbtc-chain && gosec ./...
@echo "Running Solidity security checks..."
slither contracts/ --config-file slither.config.json

# Run linters
lint:
@echo "Running Go linters..."
cd qbtc-chain && staticcheck ./...
cd qbtc-chain && golangci-lint run
@echo "Running Solidity linters..."
solhint 'contracts/**/*.sol'

# Build Docker image
docker:
docker build -t qubitcoin/qbtc-node:latest .

# Clean build artifacts
clean:
rm -rf build/ out/ cache/
