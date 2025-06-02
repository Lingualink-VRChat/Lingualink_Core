# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cli ./cmd/cli

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests, curl for health check, and ffmpeg for audio conversion
RUN apk --no-cache add ca-certificates curl ffmpeg

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/server .
COPY --from=builder /app/cli .

# Copy configuration files
COPY config/ config/

# Create non-root user
RUN adduser -D -s /bin/sh lingualink
USER lingualink

# Expose port
EXPOSE 8080

# Health check - using curl instead of wget
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

# Run the server
CMD ["./server"] 