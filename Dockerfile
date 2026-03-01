# ============================================================
# QUBITCOIN (QBTC) Node — Multi-stage Docker Build
# ============================================================
# Stage 1: Builder
# Stage 2: Runtime (minimal Alpine image)
#
# Build: docker build -t qubitcoin/qbtc-node:latest .
# Run:   docker run -p 8545:8545 -p 30303:30303 qubitcoin/qbtc-node:latest
# ============================================================

# ── Stage 1: Build ──────────────────────────────────────────
FROM golang:1.22-alpine AS builder

LABEL maintainer="Nika Hsaini <nika@qubitcoin.io>"
LABEL description="QUBITCOIN (QBTC) — Quantum-Resistant Ethereum Fork"
LABEL version="2.0.0"

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev \
    linux-headers

WORKDIR /build

# Copy go.mod and go.sum first for layer caching
COPY qbtc-chain/go.mod qbtc-chain/go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY qbtc-chain/ .

# Build the QBTC node binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
      -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') \
      -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') \
      -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /usr/local/bin/qbtc \
    ./cmd/qbtc/

# ── Stage 2: Runtime ─────────────────────────────────────────
FROM alpine:3.19 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    jq

# Create non-root user for security
RUN addgroup -g 1000 qbtc && \
    adduser -u 1000 -G qbtc -s /bin/sh -D qbtc

# Copy binary from builder
COPY --from=builder /usr/local/bin/qbtc /usr/local/bin/qbtc

# Create data directory
RUN mkdir -p /data/qbtc && chown -R qbtc:qbtc /data

# Copy default genesis configuration
COPY QubitChain_Client_Node/genesis.json /etc/qbtc/genesis.json

USER qbtc
WORKDIR /data

# ── Ports ────────────────────────────────────────────────────
# 8545: HTTP-RPC (Ethereum JSON-RPC compatible)
# 8546: WebSocket-RPC
# 30303: P2P (TCP + UDP)
# 6060: Metrics (Prometheus)
EXPOSE 8545 8546 30303/tcp 30303/udp 6060

# ── Volumes ──────────────────────────────────────────────────
VOLUME ["/data/qbtc"]

# ── Health Check ─────────────────────────────────────────────
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -sf -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        http://localhost:8545 | jq -e '.result' > /dev/null || exit 1

# ── Entrypoint ───────────────────────────────────────────────
ENTRYPOINT ["/usr/local/bin/qbtc"]
CMD [ \
    "--datadir", "/data/qbtc", \
    "--http", \
    "--http.addr", "0.0.0.0", \
    "--http.port", "8545", \
    "--http.api", "eth,net,web3,qbtc,pq", \
    "--ws", \
    "--ws.addr", "0.0.0.0", \
    "--ws.port", "8546", \
    "--metrics", \
    "--metrics.addr", "0.0.0.0", \
    "--metrics.port", "6060", \
    "--qpoa", \
    "--quantum.mining", \
    "--pq.algo", "ethfalcon" \
]
