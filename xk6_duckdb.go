// Package xk6_duckdb provides comprehensive DuckDB database functionality for k6 tests
// This extension covers the full go-duckdb API including Connector, Appender, Arrow, and Profiling
package xk6_duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	duckdb "github.com/marcboeker/go-duckdb/v2" // DuckDB driver
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

// XConnector creates a JavaScript constructor for DuckDB connectors
func (d *DuckDB) XConnector(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	return rt.ToValue(&DuckDBConnector{}).ToObject(rt)
}

// DuckDBConnection wraps a DuckDB database connection
type DuckDBConnection struct {
	db       *sql.DB
	conn     *sql.Conn
	filePath string
	config   map[string]string
}

// DuckDBConnector wraps a DuckDB connector for advanced connection management
type DuckDBConnector struct {
	connector   *duckdb.Connector
	dsn         string
	initQueries []string
}

// =============================================================================
// Basic Connection Management
// =============================================================================

// Open opens a DuckDB database connection with optional configuration
// Usage: db.open("/path/to/database.db", {threads: 4, memory_limit: "2GB"})
func (conn *DuckDBConnection) Open(dsn string, config ...map[string]interface{}) error {
	if dsn == "" {
		dsn = ":memory:"
	}
	
	// Build DSN with configuration options
	if len(config) > 0 && config[0] != nil {
		dsn += "?"
		first := true
		for key, value := range config[0] {
			if !first {
				dsn += "&"
			}
			dsn += fmt.Sprintf("%s=%v", key, value)
			first = false
		}
	}
	
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return fmt.Errorf("failed to open DuckDB: %w", err)
	}
	
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping DuckDB: %w", err)
	}
	
	conn.db = db
	conn.filePath = dsn
	
	// Store config for reference
	conn.config = make(map[string]string)
	if len(config) > 0 && config[0] != nil {
		for key, value := range config[0] {
			conn.config[key] = fmt.Sprintf("%v", value)
		}
	}
	
	return nil
}

// GetConnection returns a dedicated connection from the pool
func (conn *DuckDBConnection) GetConnection() error {
	if conn.db == nil {
		return fmt.Errorf("database is not open")
	}
	
	sqlConn, err := conn.db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	
	conn.conn = sqlConn
	return nil
}

// Close closes the database connection
func (conn *DuckDBConnection) Close() error {
	var errs []error
	
	if conn.conn != nil {
		if err := conn.conn.Close(); err != nil {
			errs = append(errs, err)
		}
		conn.conn = nil
	}
	
	if conn.db != nil {
		if err := conn.db.Close(); err != nil {
			errs = append(errs, err)
		}
		conn.db = nil
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing connection: %v", errs)
	}
	
	return nil
}

// =============================================================================
// Advanced Connector API
// =============================================================================

// NewConnector creates a new DuckDB connector with initialization callback
// Usage: const connector = new duckdb.Connector(); 
//        connector.create("test.db", ["SET memory_limit='2GB'", "SET threads=4"])
func (c *DuckDBConnector) Create(dsn string, initQueries []string) error {
	c.dsn = dsn
	c.initQueries = initQueries
	
	// Create connector with initialization function
	connector, err := duckdb.NewConnector(dsn, func(execer driver.ExecerContext) error {
		for _, query := range initQueries {
			_, err := execer.ExecContext(context.Background(), query, nil)
			if err != nil {
				return fmt.Errorf("failed to execute init query '%s': %w", query, err)
			}
		}
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}
	
	c.connector = connector
	return nil
}

// Connect creates a new connection from the connector
func (c *DuckDBConnector) Connect() (*DuckDBConnection, error) {
	if c.connector == nil {
		return nil, fmt.Errorf("connector not initialized")
	}
	
	db := sql.OpenDB(c.connector)
	
	return &DuckDBConnection{
		db:       db,
		filePath: c.dsn,
		config:   make(map[string]string),
	}, nil
}

// Close closes the connector
func (c *DuckDBConnector) Close() error {
	if c.connector != nil {
		return c.connector.Close()
	}
	return nil
}

// =============================================================================
// Query Methods
// =============================================================================

// Execute runs a SQL statement without returning results
func (conn *DuckDBConnection) Execute(query string, args ...interface{}) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	_, err := conn.db.ExecContext(context.Background(), query, args...)
	return err
}

// Query executes a SQL query and returns all results
func (conn *DuckDBConnection) Query(query string, args ...interface{}) (*QueryResult, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	rows, err := conn.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	
	return conn.processRows(rows)
}

// QuerySingle executes a query and returns only the first row
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
func (conn *DuckDBConnection) QueryScalar(query string, args ...interface{}) (interface{}, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	row := conn.db.QueryRowContext(context.Background(), query, args...)
	
	var result interface{}
	if err := row.Scan(&result); err != nil {
		return nil, fmt.Errorf("scalar query failed: %w", err)
	}
	
	return conn.convertValue(result), nil
}

// =============================================================================
// Transaction Management
// =============================================================================

// BeginTransaction starts a new transaction
func (conn *DuckDBConnection) BeginTransaction() (*DuckDBTransaction, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	tx, err := conn.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	return &DuckDBTransaction{tx: tx}, nil
}

// DuckDBTransaction wraps a SQL transaction
type DuckDBTransaction struct {
	tx *sql.Tx
}

// Execute runs a statement within the transaction
func (tx *DuckDBTransaction) Execute(query string, args ...interface{}) error {
	_, err := tx.tx.ExecContext(context.Background(), query, args...)
	return err
}

// Query runs a query within the transaction
func (tx *DuckDBTransaction) Query(query string, args ...interface{}) (*QueryResult, error) {
	rows, err := tx.tx.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return (&DuckDBConnection{}).processRows(rows)
}

// Commit commits the transaction
func (tx *DuckDBTransaction) Commit() error {
	return tx.tx.Commit()
}

// Rollback rolls back the transaction
func (tx *DuckDBTransaction) Rollback() error {
	return tx.tx.Rollback()
}

// =============================================================================
// Prepared Statements
// =============================================================================

// Prepare creates a prepared statement
func (conn *DuckDBConnection) Prepare(query string) (*DuckDBStatement, error) {
	if conn.db == nil {
		return nil, fmt.Errorf("database connection is not open")
	}
	
	stmt, err := conn.db.PrepareContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	
	return &DuckDBStatement{stmt: stmt, conn: conn}, nil
}

// DuckDBStatement wraps a prepared statement
type DuckDBStatement struct {
	stmt *sql.Stmt
	conn *DuckDBConnection
}

// Execute executes the prepared statement
func (stmt *DuckDBStatement) Execute(args ...interface{}) error {
	_, err := stmt.stmt.ExecContext(context.Background(), args...)
	return err
}

// Query executes the prepared statement and returns results
func (stmt *DuckDBStatement) Query(args ...interface{}) (*QueryResult, error) {
	rows, err := stmt.stmt.QueryContext(context.Background(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return stmt.conn.processRows(rows)
}

// Close closes the prepared statement
func (stmt *DuckDBStatement) Close() error {
	return stmt.stmt.Close()
}

// =============================================================================
// High-Performance Appender API
// =============================================================================

// CreateAppender creates a new DuckDB appender for bulk data loading
// Usage: const appender = db.createAppender("", "table_name")
func (conn *DuckDBConnection) CreateAppender(schema, table string) (*DuckDBAppender, error) {
	if conn.conn == nil {
		return nil, fmt.Errorf("need dedicated connection for appender - call getConnection() first")
	}
	
	// Get the underlying driver connection
	var driverConn driver.Conn
	err := conn.conn.Raw(func(dc interface{}) error {
		driverConn = dc.(driver.Conn)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get driver connection: %w", err)
	}
	
	appender, err := duckdb.NewAppenderFromConn(driverConn, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to create appender: %w", err)
	}
	
	return &DuckDBAppender{appender: appender}, nil
}

// DuckDBAppender wraps the DuckDB appender for high-performance bulk loading
type DuckDBAppender struct {
	appender *duckdb.Appender
}

// AppendRow appends a single row to the appender
// Usage: appender.appendRow([1, "John", "john@example.com"])
func (a *DuckDBAppender) AppendRow(values []interface{}) error {
	return a.appender.AppendRow(values...)
}

// AppendRows appends multiple rows efficiently
// Usage: appender.appendRows([[1, "John"], [2, "Jane"], [3, "Bob"]])
func (a *DuckDBAppender) AppendRows(rows [][]interface{}) error {
	for _, row := range rows {
		if err := a.appender.AppendRow(row...); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}
	return nil
}

// Flush flushes the appender buffer
func (a *DuckDBAppender) Flush() error {
	return a.appender.Flush()
}

// Close closes the appender
func (a *DuckDBAppender) Close() error {
	return a.appender.Close()
}

// =============================================================================
// Profiling API
// =============================================================================

// EnableProfiling enables query profiling on the connection
// Usage: db.enableProfiling("detailed", "no_output")
func (conn *DuckDBConnection) EnableProfiling(mode, output string) error {
	if conn.conn == nil {
		return fmt.Errorf("need dedicated connection for profiling - call getConnection() first")
	}
	
	_, err := conn.conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA enable_profiling = '%s'", output))
	if err != nil {
		return err
	}
	
	_, err = conn.conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA profiling_mode = '%s'", mode))
	return err
}

// DisableProfiling disables query profiling
func (conn *DuckDBConnection) DisableProfiling() error {
	if conn.conn == nil {
		return fmt.Errorf("need dedicated connection")
	}
	
	_, err := conn.conn.ExecContext(context.Background(), "PRAGMA disable_profiling")
	return err
}

// GetProfilingInfo retrieves profiling information for the last query
func (conn *DuckDBConnection) GetProfilingInfo() (*ProfilingInfo, error) {
	if conn.conn == nil {
		return nil, fmt.Errorf("need dedicated connection")
	}
	
	var driverConn driver.Conn
	err := conn.conn.Raw(func(dc interface{}) error {
		driverConn = dc.(driver.Conn)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get driver connection: %w", err)
	}
	
	info, err := duckdb.GetProfilingInfo(driverConn)
	if err != nil {
		return nil, fmt.Errorf("failed to get profiling info: %w", err)
	}
	
	return &ProfilingInfo{
		Query:           info.Query,
		ExecutionTime:   info.ExecutionTime.String(),
		PlanningTime:    info.PlanningTime.String(),
		OptimizationTime: info.OptimizationTime.String(),
		Metrics:         info.Metrics,
	}, nil
}

// ProfilingInfo contains query profiling information
type ProfilingInfo struct {
	Query            string                 `js:"query"`
	ExecutionTime    string                 `js:"execution_time"`
	PlanningTime     string                 `js:"planning_time"`
	OptimizationTime string                 `js:"optimization_time"`
	Metrics          map[string]interface{} `js:"metrics"`
}

// =============================================================================
// Configuration and Pragma Settings
// =============================================================================

// SetConfig sets a DuckDB configuration option
// Usage: db.setConfig("memory_limit", "4GB")
func (conn *DuckDBConnection) SetConfig(option, value string) error {
	query := fmt.Sprintf("SET %s = '%s'", option, value)
	return conn.Execute(query)
}

// GetConfig gets a DuckDB configuration option
func (conn *DuckDBConnection) GetConfig(option string) (string, error) {
	result, err := conn.QueryScalar(fmt.Sprintf("SELECT current_setting('%s')", option))
	if err != nil {
		return "", err
	}
	
	if result == nil {
		return "", nil
	}
	
	return fmt.Sprintf("%v", result), nil
}

// SetPragma sets a DuckDB pragma
// Usage: db.setPragma("threads", "8")
func (conn *DuckDBConnection) SetPragma(pragma, value string) error {
	query := fmt.Sprintf("PRAGMA %s = %s", pragma, value)
	return conn.Execute(query)
}

// =============================================================================
// Extension Management
// =============================================================================

// LoadExtension loads a DuckDB extension
// Usage: db.loadExtension("json") or db.loadExtension("/path/to/extension.duckdb_extension")
func (conn *DuckDBConnection) LoadExtension(extension string) error {
	query := fmt.Sprintf("LOAD '%s'", extension)
	return conn.Execute(query)
}

// InstallExtension installs a DuckDB extension
func (conn *DuckDBConnection) InstallExtension(extension string) error {
	query := fmt.Sprintf("INSTALL '%s'", extension)
	return conn.Execute(query)
}

// =============================================================================
// Data Management (Enhanced)
// =============================================================================

// CreateTable creates a table with the specified structure
func (conn *DuckDBConnection) CreateTable(tableName string, columns map[string]string) error {
	if conn.db == nil {
		return fmt.Errorf("database connection is not open")
	}
	
	if len(columns) == 0 {
		return fmt.Errorf("no columns specified")
	}
	
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

// InsertData inserts data into a table (uses transactions for better performance)
func (conn *DuckDBConnection) InsertData(tableName string, data []map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}
	
	tx, err := conn.BeginTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Get column names from the first row
	var columns []string
	for col := range data[0] {
		columns = append(columns, col)
	}
	
	// Prepare the INSERT statement
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "))
	
	for _, row := range data {
		values := make([]interface{}, len(columns))
		for i, col := range columns {
			values[i] = row[col]
		}
		
		if err := tx.Execute(query, values...); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}
	}
	
	return tx.Commit()
}

// LoadCSV loads data from a CSV file using DuckDB's native CSV loader
func (conn *DuckDBConnection) LoadCSV(tableName, filePath string, options map[string]interface{}) error {
	query := fmt.Sprintf("COPY %s FROM '%s' (FORMAT CSV", tableName, filePath)
	
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

// LoadParquet loads data from a Parquet file
func (conn *DuckDBConnection) LoadParquet(tableName, filePath string) error {
	query := fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM parquet_scan('%s')", tableName, filePath)
	return conn.Execute(query)
}

// LoadJSON loads data from a JSON file
func (conn *DuckDBConnection) LoadJSON(tableName, filePath string, options map[string]interface{}) error {
	query := fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM read_json_auto('%s'", tableName, filePath)
	
	if format, ok := options["format"]; ok {
		query += fmt.Sprintf(", format='%s'", format)
	}
	
	query += ")"
	return conn.Execute(query)
}

// =============================================================================
// Helper Methods and Data Processing
// =============================================================================

// processRows processes SQL rows into QueryResult
func (conn *DuckDBConnection) processRows(rows *sql.Rows) (*QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	var results []map[string]interface{}
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = conn.convertValue(values[i])
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

// convertValue converts Go values to JavaScript-friendly types
func (conn *DuckDBConnection) convertValue(val interface{}) interface{} {
	switch v := val.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case nil:
		return nil
	default:
		return v
	}
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