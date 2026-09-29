# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build statically linked binaries for API and Worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/bnpl-api \
    ./cmd/api/main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/bnpl-worker \
    ./cmd/worker/main.go

# Production minimal runtime stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -S ledgerly && adduser -S -G ledgerly ledgerly
USER ledgerly:ledgerly

COPY --from=builder /app/bin/bnpl-api /app/bnpl-api
COPY --from=builder /app/bin/bnpl-worker /app/bnpl-worker

EXPOSE 8081

HEALTHCHECK --interval=5s --timeout=3s --retries=3 \
  CMD curl -f http://localhost:8081/healthz || exit 1

ENTRYPOINT ["/app/bnpl-api"]
