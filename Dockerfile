# Multi-stage build for xiaozhi-server-go
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies for CGO (SQLite driver requires gcc)
RUN apk add --no-cache gcc musl-dev

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Build binary with CGO enabled
ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w" -o xiaozhi-server ./src/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite-libs \
    procps

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/xiaozhi-server /app/xiaozhi-server

# Copy assets, audio files, and configuration template
COPY web /app/web
COPY music /app/music
COPY config.yaml /app/config.yaml
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh /app/xiaozhi-server

# Create data directory for volume mounting
RUN mkdir -p /app/data

EXPOSE 8000 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["./xiaozhi-server"]
