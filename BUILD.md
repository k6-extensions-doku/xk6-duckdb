# Building k6 DuckDB Extension - Step by Step

This guide will help you build and test the k6 DuckDB extension, addressing common version compatibility issues.

## Prerequisites

### 1. Install Required Tools

```bash
# Install Go (1.21 or later)
# Download from https://golang.org/dl/

# Install xk6
go install go.k6.io/xk6/cmd/xk6@latest

# Install build tools (Linux/Ubuntu)
sudo apt-get update && sudo apt-get install build-essential

# Install build tools (macOS)
xcode-select --install

# Install build tools (Windows - using MSYS2)
# Download and install MSYS2 from https://www.msys2.org/
# Then run in MSYS2 terminal:
pacman -S mingw-w64-ucrt-x86_64-gcc
```

### 2. Verify Installation

```bash
go version        # Should show Go 1.21+
xk6 version      # Should show xk6 version
gcc --version    # Should show GCC compiler
```

## Quick Start (Minimal Version)

If you want to get started quickly with basic functionality:

### 1. Create Project Structure

```bash
mkdir xk6-duckdb && cd xk6-duckdb
```

### 2. Create go.mod

```go
module github.com/k6-extensions-doku/xk6-duckdb

go 1.21

require (
    github.com/marcboeker/go-duckdb/v2 v2.3.3
    go.k6.io/k6 v0.50.0
)
```

### 3. Use the Minimal Extension Code

Copy the `minimal-duckdb.go` code (provided above) as your main extension file.

### 4. Build

```bash
# Initialize go modules
go mod tidy

# Build with xk6
CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

### 5. Test

```bash
# Run the simple test
./k6 run test.js
```

## Advanced Build (Full Featured Version)

For the complete extension with all features:

### 1. Handle Version Dependencies

The sobek dependency is automatically resolved by xk6. If you encounter version issues:

```bash
# Let xk6 handle dependencies automatically
go mod edit -droprequire=github.com/grafana/sobek

# Clean up
go clean -modcache
go mod tidy
```

### 2. Build with Verbose Output

```bash
CGO_ENABLED=1 xk6 build -v --with github.com/k6-extensions-doku/xk6-duckdb=.
```

## Troubleshooting Common Issues

### Issue 1: "invalid version: unknown revision"

**Problem**: Specific version commits don't exist
**Solution**:
```bash
# Remove specific version constraints
go mod edit -droprequire=github.com/grafana/sobek
go mod tidy

# Let xk6 resolve dependencies
CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

### Issue 2: "undefined: conn" 

**Problem**: CGO is disabled
**Solution**:
```bash
# Explicitly enable CGO
export CGO_ENABLED=1

# On Windows, ensure GCC is in PATH
export PATH="/c/msys64/ucrt64/bin:$PATH"  # Windows Git Bash
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"  # Windows PowerShell
```

### Issue 3: Build Tools Missing

**Ubuntu/Debian**:
```bash
sudo apt-get update
sudo apt-get install build-essential gcc
```

**CentOS/RHEL**:
```bash
sudo yum groupinstall "Development Tools"
sudo yum install gcc
```

**macOS**:
```bash
xcode-select --install
```

### Issue 4: Cross-Compilation Issues

```bash
# For cross-compilation, set the cross-compiler
CC=x86_64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=amd64 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

## Platform-Specific Instructions

### macOS (ARM64/M1/M2)

```bash
# Ensure you have the right architecture
GOARCH=arm64 CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

### Windows

```bash
# Using PowerShell
$env:CGO_ENABLED = "1"
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.

# Using Git Bash
export CGO_ENABLED=1
export PATH="/c/msys64/ucrt64/bin:$PATH"
xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

### Linux

```bash
# Standard build
CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.

# For older systems, you might need:
CGO_CFLAGS="-DDUCKDB_STATIC_BUILD" CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

## Testing Your Build

### 1. Quick Verification

```bash
# Check if the extension loads
./k6 run --no-vu-connection-reuse -e 'import duckdb from "k6/x/duckdb"; console.log("DuckDB loaded successfully");'
```

### 2. Run Basic Tests

```bash
# Run the simple test
./k6 run test.js

# Run with more verbose output
./k6 run --verbose test.js
```

### 3. Run Load Tests

```bash
# Small load test
./k6 run --vus 5 --iterations 10 test.js

# Stress test
./k6 run --vus 10 --duration 30s test.js
```

## Docker Build (Alternative)

If you're having trouble with local builds:

```bash
# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git build-base gcc musl-dev
RUN go install go.k6.io/xk6/cmd/xk6@latest

WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=1 xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.

FROM alpine:3.18
RUN apk add --no-cache ca-certificates libc6-compat
COPY --from=builder /app/k6 /usr/local/bin/k6
ENTRYPOINT ["k6"]
EOF

# Build Docker image
docker build -t k6-duckdb .

# Run test in Docker
docker run --rm -v $(pwd):/tests k6-duckdb run /tests/test.js
```

## Next Steps

Once you have a successful build:

1. **Upload to GitHub**: Push your extension to a GitHub repository
2. **Version tagging**: Use Git tags for version management
3. **CI/CD**: Set up GitHub Actions for automated builds
4. **Documentation**: Create comprehensive docs for your users
5. **Testing**: Add comprehensive test suites

## Getting Help

If you encounter issues:

1. Check the [k6 extension documentation](https://k6.io/docs/extensions/)
2. Review [go-duckdb issues](https://github.com/marcboeker/go-duckdb/issues)
3. Check [xk6 GitHub issues](https://github.com/grafana/xk6/issues)
4. Join the [k6 Community](https://community.k6.io/)

## Version Compatibility Matrix

| k6 Version | go-duckdb Version | Go Version | Notes |
|------------|-------------------|------------|-------|
| v0.50.x    | v2.3.3           | 1.21+      | Latest stable |
| v0.49.x    | v2.3.x           | 1.21+      | Compatible |
| v0.48.x    | v2.2.x           | 1.20+      | Older stable |

Always use the latest compatible versions for the best experience.