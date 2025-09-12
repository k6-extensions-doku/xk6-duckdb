# k6 DuckDB Extension

A k6 extension that provides DuckDB database functionality for load testing scenarios involving database operations. This extension leverages the [go-duckdb](https://github.com/marcboeker/go-duckdb) library to provide high-performance analytical database capabilities within k6 tests.

## Features

- **In-memory and persistent databases**: Support for both temporary and file-based databases
- **Full SQL support**: Execute any DuckDB-compatible SQL statements
- **High performance**: Built on DuckDB's columnar engine optimized for analytics
- **Easy data loading**: Built-in support for CSV loading and batch insertions
- **Go-to-JavaScript bridge**: Seamless integration with k6's JavaScript runtime
- **Connection management**: Proper resource management with connection pooling

## Installation

### Prerequisites

- Go 1.21 or later
- xk6 tool for building k6 with extensions
- GCC compiler (required for go-duckdb)

### Build the Extension

1. Install xk6:
```bash
go install go.k6.io/xk6/cmd/xk6@latest
```

2. Clone or create your extension directory:
```bash
mkdir xk6-duckdb && cd xk6-duckdb
```

3. Create the files (go.mod, main.go) as provided in this example

4. Build k6 with the DuckDB extension:
```bash
xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

This will create a `k6` binary in your current directory with the DuckDB extension included.

## Usage

### Basic Connection and Queries

```javascript
import duckdb from 'k6/x/duckdb';

export default function() {
    const db = new duckdb.DuckDB();
    
    try {
        // Open database (empty string = in-memory)
        db.open("");
        
        // Create table
        db.createTable("users", {
            id: "INTEGER PRIMARY KEY",
            name: "VARCHAR(100)",
            email: "VARCHAR(100)"
        });
        
        // Insert data
        db.insertData("users", [
            { id: 1, name: "John Doe", email: "john@example.com" },
            { id: 2, name: "Jane Smith", email: "jane@example.com" }
        ]);
        
        // Query data
        const users = db.query("SELECT * FROM users WHERE id > ?", [0]);
        console.log(`Found ${users.count} users`);
        
        // Get single row
        const user = db.querySingle("SELECT * FROM users WHERE id = ?", [1]);
        console.log(`User: ${user.name}`);
        
        // Get scalar value
        const count = db.queryScalar("SELECT COUNT(*) FROM users");
        console.log(`Total users: ${count}`);
        
    } finally {
        db.close();
    }
}
```

### Advanced Features

```javascript
// Load CSV data
db.loadCSV("sales_data", "/path/to/sales.csv", {
    header: true,
    delimiter: ",",
    quote: "\""
});

// Complex analytics queries
const analytics = db.query(`
    SELECT 
        DATE_TRUNC('month', order_date) as month,
        COUNT(*) as order_count,
        SUM(amount) as total_revenue,
        AVG(amount) as avg_order_value
    FROM orders 
    WHERE order_date >= '2024-01-01'
    GROUP BY DATE_TRUNC('month', order_date)
    ORDER BY month
`);

// Use DuckDB's built-in functions
const stats = db.query(`
    SELECT 
        percentile_cont(0.5) WITHIN GROUP (ORDER BY amount) as median_amount,
        stddev(amount) as amount_stddev
    FROM orders
`);
```

## API Reference

### DuckDB Class

#### Constructor
- `new duckdb.DuckDB()` - Creates a new DuckDB connection instance

#### Methods

##### Connection Management
- `open(dsn: string): void` - Opens database connection
  - `dsn`: Database path or empty string for in-memory database
  - Example: `db.open("/path/to/database.db")` or `db.open("")`

- `close(): void` - Closes the database connection

##### Query Methods
- `query(sql: string, ...args): QueryResult` - Executes query and returns all results
- `querySingle(sql: string, ...args): object|null` - Returns first row only
- `queryScalar(sql: string, ...args): any` - Returns single value
- `execute(sql: string, ...args): void` - Executes statement without returning results

##### Data Management
- `createTable(name: string, columns: object): void` - Creates table with specified columns
- `insertData(tableName: string, rows: array): void` - Bulk insert data
- `loadCSV(tableName: string, filePath: string, options: object): void` - Load CSV file

### QueryResult Object

```javascript
{
    rows: [],        // Array of result rows as objects
    columns: [],     // Array of column names
    count: 0         // Number of rows returned
}
```

### Field Name Conventions

Following k6's Go-to-JavaScript bridge conventions:
- Go method names (PascalCase) become camelCase in JavaScript: `QuerySingle` → `querySingle`
- Go struct fields become snake_case in JavaScript: `RowCount` → `row_count`

## Performance Considerations

### Best Practices

1. **Connection Management**: Always close connections in `finally` blocks
2. **Batch Operations**: Use `insertData()` for bulk inserts instead of individual INSERT statements
3. **In-Memory vs Persistent**: Use in-memory databases for temporary test data, persistent for shared data
4. **Query Optimization**: Leverage DuckDB's columnar engine for analytical queries
5. **Resource Cleanup**: Use setup/teardown functions for database initialization and cleanup

### Memory Usage

DuckDB runs in-process, so all data lives in your k6 process memory:
- In-memory databases: All data stored in RAM
- Persistent databases: Uses memory for caching and processing
- Monitor memory usage with large datasets

## Example Use Cases

### 1. Testing Data Pipeline Performance
```javascript
export default function() {
    const db = new duckdb.DuckDB();
    db.open("");
    
    // Simulate ETL process
    const start = Date.now();
    db.loadCSV("raw_data", "input.csv", {header: true});
    
    db.execute(`
        CREATE TABLE processed_data AS
        SELECT 
            user_id,
            DATE_TRUNC('day', timestamp) as date,
            SUM(value) as daily_total
        FROM raw_data
        GROUP BY user_id, DATE_TRUNC('day', timestamp)
    `);
    
    const processingTime = Date.now() - start;
    console.log(`ETL completed in ${processingTime}ms`);
}
```

### 2. Database Load Testing
```javascript
export const options = {
    scenarios: {
        read_heavy: {
            executor: 'constant-vus',
            vus: 50,
            duration: '5m',
        },
    },
};

export default function() {
    const db = new duckdb.DuckDB();
    db.open("shared_test.db");
    
    // Simulate read-heavy workload
    const userId = Math.floor(Math.random() * 10000) + 1;
    const orders = db.query(`
        SELECT * FROM orders 
        WHERE user_id = ? AND order_date >= CURRENT_DATE - INTERVAL '30 days'
        ORDER BY order_date DESC
    `, [userId]);
    
    check(orders, {
        'Query executed successfully': (r) => r.count >= 0,
        'Response time acceptable': () => true,
    });
    
    db.close();
}
```

### 3. Analytics Workload Testing
```javascript
export function setup() {
    const db = new duckdb.DuckDB();
    db.open("analytics_test.db");
    
    // Generate test data
    db.execute(`
        CREATE TABLE events AS
        SELECT 
            (random() * 1000000)::INTEGER as user_id,
            ['login', 'purchase', 'view', 'click'][1 + (random() * 3)::INTEGER] as event_type,
            random() * 100 as value,
            CURRENT_TIMESTAMP - INTERVAL (random() * 365) DAY as timestamp
        FROM generate_series(1, 1000000)
    `);
    
    db.close();
    return { dbPath: "analytics_test.db" };
}

export default function(data) {
    const db = new duckdb.DuckDB();
    db.open(data.dbPath);
    
    // Complex analytical query
    const analytics = db.query(`
        WITH daily_stats AS (
            SELECT 
                DATE_TRUNC('day', timestamp) as date,
                event_type,
                COUNT(*) as event_count,
                SUM(value) as total_value
            FROM events
            WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'
            GROUP BY DATE_TRUNC('day', timestamp), event_type
        ),
        user_segments AS (
            SELECT 
                user_id,
                COUNT(DISTINCT event_type) as event_types,
                SUM(value) as lifetime_value,
                CASE 
                    WHEN SUM(value) > 500 THEN 'high_value'
                    WHEN SUM(value) > 100 THEN 'medium_value'
                    ELSE 'low_value'
                END as segment
            FROM events
            GROUP BY user_id
        )
        SELECT 
            ds.date,
            ds.event_type,
            ds.event_count,
            us.segment,
            COUNT(DISTINCT us.user_id) as unique_users
        FROM daily_stats ds
        JOIN events e ON DATE_TRUNC('day', e.timestamp) = ds.date 
                     AND e.event_type = ds.event_type
        JOIN user_segments us ON e.user_id = us.user_id
        GROUP BY ds.date, ds.event_type, ds.event_count, us.segment
        ORDER BY ds.date, ds.event_type
    `);
    
    console.log(`Processed ${analytics.count} analytical results`);
    db.close();
}
```

## Error Handling

Always wrap database operations in try-catch blocks:

```javascript
export default function() {
    const db = new duckdb.DuckDB();
    
    try {
        db.open("");
        
        // Your database operations here
        const result = db.query("SELECT * FROM non_existent_table");
        
    } catch (error) {
        console.error('Database error:', error.message);
        
        // Log error for k6 metrics
        check(false, {
            'Database operation successful': () => false,
        });
        
    } finally {
        // Always close connection
        try {
            db.close();
        } catch (closeError) {
            console.error('Error closing database:', closeError);
        }
    }
}
```

## Troubleshooting

### Common Issues

#### Build Errors

**Error: `undefined: conn`**
```bash
# Install build tools
sudo apt-get update && sudo apt-get install build-essential

# Enable CGO
export CGO_ENABLED=1
xk6 build --with github.com/k6-extensions-doku/xk6-duckdb=.
```

**Windows Build Issues**
```bash
# Install MSYS2 and GCC
pacman -S mingw-w64-ucrt-x86_64-gcc

# Add to PATH
$env:PATH = "C:\msys64\ucrt64\bin:$env:PATH"
```

#### Runtime Errors

**Database Connection Issues**
- Ensure proper file permissions for persistent databases
- Check disk space for large datasets
- Verify database file paths are accessible

**Memory Issues**
- Monitor memory usage with large in-memory databases
- Consider using persistent storage for large datasets
- Implement proper connection cleanup

### Performance Tuning

#### DuckDB Configuration
```javascript
// Set DuckDB configuration options
db.open("test.db?threads=4&memory_limit=2GB");

// Or configure after opening
db.execute("SET memory_limit='2GB'");
db.execute("SET threads=4");
```

#### Query Optimization
```javascript
// Use EXPLAIN to understand query plans
const plan = db.query("EXPLAIN SELECT * FROM large_table WHERE condition = 'value'");
console.log('Query plan:', JSON.stringify(plan.rows, null, 2));

// Create indexes for better performance
db.execute("CREATE INDEX idx_user_id ON orders(user_id)");
db.execute("CREATE INDEX idx_timestamp ON events(timestamp)");
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Related Projects

- [k6](https://k6.io/) - Modern load testing tool
- [go-duckdb](https://github.com/marcboeker/go-duckdb) - DuckDB driver for Go
- [DuckDB](https://duckdb.org/) - Analytical SQL database engine
- [xk6](https://github.com/grafana/xk6) - Extension system for k6

## Support

- [k6 Community Forum](https://community.k6.io/)
- [DuckDB Documentation](https://duckdb.org/docs/)
- [GitHub Issues](https://github.com/k6-extensions-doku/xk6-duckdb/issues)