# Build stage - use specific version for reproducibility
FROM golang:1.24-alpine AS builder

# Install ca-certificates for HTTPS downloads
RUN apk add --no-cache ca-certificates

# Create appuser
RUN adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o aws-ghost ./cmd/aws-ghost/cmd

# Final stage - minimal scratch image
FROM scratch

# Import CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import user and group files from builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the binary from builder stage
COPY --from=builder /app/aws-ghost /aws-ghost

# Use non-root user
USER appuser:appuser

# Set entrypoint
ENTRYPOINT ["/aws-ghost"]
