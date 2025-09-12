// Package xk6_duckdb provides DuckDB database functionality for k6 tests
package xk6_duckdb

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb/v2" // DuckDB driver
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
)

// init registers the DuckDB module with k6
func init() {
	modules.Register("k6/x/duckdb", New())
}

// DuckDB represents the main module struct
type DuckDB struct{}

// New creates a new instance of the DuckDB module
func New() *DuckDB {
	return &DuckDB{}
}

// XDuckDB creates a JavaScript constructor for DuckDB connections
func (d *DuckDB) XDuckDB(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	return rt.ToValue(&DuckDBConnection{}).ToObject(rt)
}

// DuckDBConnection wraps a DuckDB database connection
type DuckDBConnection struct {
	db       *sql.DB
	filePath string
}

// Open opens a DuckDB database connection
// Usage in JS: const db = new duckdb.DuckDB(); db.open("/path/to/database.db")
func (conn *DuckDBConnection) Open(dsn string) error {
	if dsn == "" {
		dsn = ":memory:" // In-memory database if no path provided
	}
	
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return fmt.Errorf("failed to open DuckDB: %w", err)
	}
	
	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping DuckDB: %w", err)
	}
	
	conn.db = db
	conn.filePath = dsn
	return nil
}

// Close closes the database connection
func (conn *DuckDBConnection) Close() error {
	if conn.db == nil {
		return nil
	}
	
	err := conn.db.Close()
	conn.db = nil
	return err
}

// Execute runs a SQL statement without returning results
// Usage in JS: db.execute("CREATE TABLE users (id INTEGER, name TEXT)")
func (conn *DuckDBConnection) Execute(query string) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	_, err := conn.db.ExecContext(context.Background(), query)
	return err
}

// Query executes a SQL query and returns the results
// Usage in JS: const result = db.query("SELECT * FROM users WHERE id = ?", [1])
func (conn *DuckDBConnection) Query(query string, args ...interface{}) (*QueryResult, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	rows, err := conn.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	var results []map[string]interface{}
	
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			
			// Convert []byte to string for better JS compatibility
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			
			row[col] = val
		}
		
		results = append(results, row)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	
	return &QueryResult{
		Rows:    results,
		Columns: columns,
		Count:   len(results),
	}, nil
}

// QuerySingle executes a query and returns only the first row
// Usage in JS: const user = db.querySingle("SELECT * FROM users WHERE id = ?", [1])
func (conn *DuckDBConnection) QuerySingle(query string, args ...interface{}) (map[string]interface{}, error) {
	result, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	
	if result.Count == 0 {
		return nil, nil
	}
	
	return result.Rows[0], nil
}

// QueryScalar executes a query and returns a single scalar value
// Usage in JS: const count = db.queryScalar("SELECT COUNT(*) FROM users")
func (conn *DuckDBConnection) QueryScalar(query string, args ...interface{}) (interface{}, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	row := conn.db.QueryRowContext(context.Background(), query, args...)
	
	var result interface{}
	if err := row.Scan(&result); err != nil {
		return nil, fmt.Errorf("scalar query failed: %w", err)
	}
	
	// Convert []byte to string for better JS compatibility
	if b, ok := result.([]byte); ok {
		result = string(b)
	}
	
	return result, nil
}

// CreateTable creates a table with the specified structure
// Usage in JS: db.createTable("users", {id: "INTEGER PRIMARY KEY", name: "TEXT", email: "TEXT"})
func (conn *DuckDBConnection) CreateTable(tableName string, columns map[string]string) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	if len(columns) == 0 {
		return fmt.Errorf("no columns specified")
	}
	
	// Build CREATE TABLE statement
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", tableName)
	
	i := 0
	for colName, colType := range columns {
		if i > 0 {
			query += ", "
		}
		query += fmt.Sprintf("%s %s", colName, colType)
		i++
	}
	
	query += ")"
	
	return conn.Execute(query)
}

// InsertData inserts data into a table
// Usage in JS: db.insertData("users", [{id: 1, name: "John", email: "john@example.com"}])
func (conn *DuckDBConnection) InsertData(tableName string, data []map[string]interface{}) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	if len(data) == 0 {
		return nil // Nothing to insert
	}
	
	// Get column names from the first row
	var columns []string
	for col := range data[0] {
		columns = append(columns, col)
	}
	
	// Begin transaction for batch insert
	tx, err := conn.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Prepare the INSERT statement
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "))
	
	stmt, err := tx.PrepareContext(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	
	// Insert each row
	for _, row := range data {
		values := make([]interface{}, len(columns))
		for i, col := range columns {
			values[i] = row[col]
		}
		
		_, err := stmt.ExecContext(context.Background(), values...)
		if err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}
	}
	
	return tx.Commit()
}

// LoadCSV loads data from a CSV file using DuckDB's native CSV loader
// Usage in JS: db.loadCSV("users", "/path/to/users.csv", {header: true, delimiter: ","})
func (conn *DuckDBConnection) LoadCSV(tableName, filePath string, options map[string]interface{}) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	// Build the COPY statement with options
	query := fmt.Sprintf("COPY %s FROM '%s' (FORMAT CSV", tableName, filePath)
	
	// Add options
	if header, ok := options["header"]; ok && header == true {
		query += ", HEADER"
	}
	
	if delimiter, ok := options["delimiter"]; ok {
		if delim, ok := delimiter.(string); ok {
			query += fmt.Sprintf(", DELIMITER '%s'", delim)
		}
	}
	
	if quote, ok := options["quote"]; ok {
		if q, ok := quote.(string); ok {
			query += fmt.Sprintf(", QUOTE '%s'", q)
		}
	}
	
	query += ")"
	
	return conn.Execute(query)
}

// QueryResult represents the result of a database query
type QueryResult struct {
	Rows    []map[string]interface{} `js:"rows"`
	Columns []string                 `js:"columns"`
	Count   int                      `js:"count"`
}

// Helper function to join strings
func joinStrings(strs []string, separator string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += separator + strs[i]
	}
	return result
}