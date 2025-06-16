# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o webtunnel ./cmd/webtunnel

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates bash curl

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/webtunnel .

# Copy static files (if any)
COPY --from=builder /app/web/dist ./web/dist

# Create sessions directory
RUN mkdir -p /tmp/webtunnel/sessions

# Expose port
EXPOSE 8443

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f https://localhost:8443/health || exit 1

# Run the binary
CMD ["./webtunnel", "serve"]