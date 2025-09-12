# k6 DuckDB Extension Makefile

# Variables
BINARY_NAME := k6
XK6_VERSION := latest
MODULE_NAME := github.com/k6-extensions-doku/xk6-duckdb
BUILD_DIR := build
TEST_SCRIPT := test.js

# Default target
.DEFAULT_GOAL := build

# Install xk6 if not present
install-xk6:
	@which xk6 > /dev/null || go install go.k6.io/xk6/cmd/xk6@$(XK6_VERSION)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f k6.exe

# Build the k6 binary with DuckDB extension
build: install-xk6 clean
	@echo "Building k6 with DuckDB extension..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 xk6 build \
		--output $(BUILD_DIR)/$(BINARY_NAME) \
		--with $(MODULE_NAME)=.

# Build for different platforms
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 xk6 build \
		--output $(BUILD_DIR)/$(BINARY_NAME)-linux \
		--with $(MODULE_NAME)=.

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 xk6 build \
		--output $(BUILD_DIR)/$(BINARY_NAME)-darwin \
		--with $(MODULE_NAME)=.

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 xk6 build \
		--output $(BUILD_DIR)/$(BINARY_NAME).exe \
		--with $(MODULE_NAME)=.

# Run tests using the built binary
test: build
	@echo "Running k6 tests with DuckDB extension..."
	@if [ -f $(TEST_SCRIPT) ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) run $(TEST_SCRIPT); \
	else \
		echo "Test script $(TEST_SCRIPT) not found!"; \
		exit 1; \
	fi

# Run tests with specific options
test-load: build
	@echo "Running load test..."
	$(BUILD_DIR)/$(BINARY_NAME) run --vus 10 --iterations 100 $(TEST_SCRIPT)

test-stress: build
	@echo "Running stress test..."
	$(BUILD_DIR)/$(BINARY_NAME) run --vus 50 --duration 30s $(TEST_SCRIPT)

# Development helpers
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

lint: 
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run

# Check dependencies
deps:
	@echo "Checking dependencies..."
	@go mod verify
	@go mod tidy

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	@go get -u github.com/marcboeker/go-duckdb/v2
	@go get -u go.k6.io/k6
	@go get -u github.com/grafana/sobek
	@go mod tidy

# Create sample test files
samples: 
	@echo "Creating sample test files..."
	@mkdir -p examples
	@echo "Creating basic example..."
	@cat > examples/basic.js << 'EOF'
import duckdb from 'k6/x/duckdb';

export default function() {
    const db = new duckdb.DuckDB();
    
    try {
        db.open("");
        console.log("Database opened successfully!");
        
        // Create a simple table
        db.createTable("test", {
            id: "INTEGER PRIMARY KEY",
            name: "TEXT"
        });
        
        // Insert some data
        db.insertData("test", [
            { id: 1, name: "Hello" },
            { id: 2, name: "World" }
        ]);
        
        // Query the data
        const results = db.query("SELECT * FROM test ORDER BY id");
        console.log(`Found ${results.count} rows`);
        
        for (const row of results.rows) {
            console.log(`ID: ${row.id}, Name: ${row.name}`);
        }
        
    } finally {
        db.close();
    }
}
EOF

	@echo "Creating CSV loading example..."
	@cat > examples/csv-example.js << 'EOF'
import duckdb from 'k6/x/duckdb';

export function setup() {
    // Create sample CSV data
    const csvData = `id,name,email,age
1,John Doe,john@example.com,30
2,Jane Smith,jane@example.com,25
3,Bob Johnson,bob@example.com,35`;
    
    console.log("Sample CSV data created");
    return { csvData };
}

export default function(data) {
    const db = new duckdb.DuckDB();
    
    try {
        db.open("");
        
        // Create table for CSV data
        db.createTable("users", {
            id: "INTEGER",
            name: "TEXT", 
            email: "TEXT",
            age: "INTEGER"
        });
        
        // In a real scenario, you would load from a file:
        // db.loadCSV("users", "/path/to/users.csv", {header: true});
        
        // For this example, we'll insert the data manually
        db.insertData("users", [
            { id: 1, name: "John Doe", email: "john@example.com", age: 30 },
            { id: 2, name: "Jane Smith", email: "jane@example.com", age: 25 },
            { id: 3, name: "Bob Johnson", email: "bob@example.com", age: 35 }
        ]);
        
        // Run some analytics
        const avgAge = db.queryScalar("SELECT AVG(age) FROM users");
        console.log(`Average age: ${avgAge}`);
        
        const usersByAge = db.query(`
            SELECT 
                CASE 
                    WHEN age < 30 THEN 'Young'
                    WHEN age < 40 THEN 'Middle'
                    ELSE 'Senior'
                END as age_group,
                COUNT(*) as count
            FROM users 
            GROUP BY age_group
            ORDER BY count DESC
        `);
        
        console.log("Users by age group:", JSON.stringify(usersByAge.rows));
        
    } finally {
        db.close();
    }
}
EOF

	@echo "Sample files created in examples/ directory"

# Docker support
docker-build:
	@echo "Building Docker image..."
	@docker build -t k6-duckdb .

# Help target
help:
	@echo "Available targets:"
	@echo "  build         - Build k6 binary with DuckDB extension"
	@echo "  build-linux   - Build for Linux"
	@echo "  build-darwin  - Build for macOS" 
	@echo "  build-windows - Build for Windows"
	@echo "  test          - Run basic tests"
	@echo "  test-load     - Run load test (10 VUs, 100 iterations)"
	@echo "  test-stress   - Run stress test (50 VUs, 30s duration)"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run linter"
	@echo "  deps          - Check dependencies"
	@echo "  update-deps   - Update dependencies"
	@echo "  samples       - Create sample test files"
	@echo "  docker-build  - Build Docker image"
	@echo "  help          - Show this help"

.PHONY: build build-linux build-darwin build-windows test test-load test-stress clean fmt lint deps update-deps samples docker-build help install-xk6