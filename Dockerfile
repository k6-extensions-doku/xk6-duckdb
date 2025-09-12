# Multi-stage build for k6 with DuckDB extension

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    build-base \
    gcc \
    musl-dev

# Set working directory
WORKDIR /app

# Install xk6
RUN go install go.k6.io/xk6/cmd/xk6@latest

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build k6 with DuckDB extension
RUN CGO_ENABLED=1 xk6 build \
    --output k6 \
    --with github.com/k6-extensions-doku/xk6-duckdb=.

# Stage 2: Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    libc6-compat

# Create non-root user
RUN adduser -D -s /bin/sh k6user

# Copy the built binary
COPY --from=builder /app/k6 /usr/local/bin/k6

# Make sure the binary is executable
RUN chmod +x /usr/local/bin/k6

# Create directories for tests and data
RUN mkdir -p /home/k6user/tests /home/k6user/data && \
    chown -R k6user:k6user /home/k6user

# Switch to non-root user
USER k6user
WORKDIR /home/k6user

# Set environment variables
ENV K6_WEB_DASHBOARD=true
ENV K6_WEB_DASHBOARD_EXPORT=/home/k6user/data/report.html

# Expose port for web dashboard
EXPOSE 5665

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD k6 version || exit 1

# Default command
ENTRYPOINT ["k6"]
CMD ["--help"]

# Labels for better container management
LABEL maintainer="your-email@example.com"
LABEL description="k6 load testing tool with DuckDB extension"
LABEL version="1.0.0"

# Usage examples:
# Build: docker build -t k6-duckdb .
# Run test: docker run --rm -v $(pwd)/tests:/home/k6user/tests k6-duckdb run /home/k6user/tests/test.js
# Interactive: docker run --rm -it k6-duckdb sh