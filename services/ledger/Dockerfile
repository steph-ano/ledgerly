# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/ledger \
    ./cmd/api/main.go

# Production minimal runtime stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -S ledgerly && adduser -S -G ledgerly ledgerly
USER ledgerly:ledgerly

COPY --from=builder /app/bin/ledger /app/ledger

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/ledger"]
