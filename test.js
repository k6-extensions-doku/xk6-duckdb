import { check } from 'k6';
import duckdb from 'k6/x/duckdb';

export const options = {
  scenarios: {
    database_operations: {
      executor: 'shared-iterations',
      vus: 10,
      iterations: 100,
    },
  },
};

// Setup function runs once before all iterations
export function setup() {
  console.log('Setting up DuckDB for load testing...');
  
  const db = new duckdb.DuckDB();
  
  try {
    // Open in-memory database (or use a file path like "test.db")
    db.open("");
    
    // Create tables
    db.createTable("users", {
      id: "INTEGER PRIMARY KEY",
      name: "VARCHAR(100)",
      email: "VARCHAR(100)",
      created_at: "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
    });
    
    db.createTable("orders", {
      id: "INTEGER PRIMARY KEY", 
      user_id: "INTEGER",
      product: "VARCHAR(100)",
      amount: "DECIMAL(10,2)",
      order_date: "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
    });
    
    // Insert some initial test data
    const users = [
      { id: 1, name: "John Doe", email: "john@example.com" },
      { id: 2, name: "Jane Smith", email: "jane@example.com" },
      { id: 3, name: "Bob Johnson", email: "bob@example.com" }
    ];
    
    const orders = [
      { id: 1, user_id: 1, product: "Laptop", amount: 1200.50 },
      { id: 2, user_id: 2, product: "Phone", amount: 800.00 },
      { id: 3, user_id: 1, product: "Mouse", amount: 25.99 }
    ];
    
    db.insertData("users", users);
    db.insertData("orders", orders);
    
    console.log('Database setup completed successfully');
    return { dbPath: "" }; // Return setup data for VUs
    
  } catch (error) {
    console.error('Setup failed:', error);
    throw error;
  } finally {
    db.close();
  }
}

// Main test function - runs for each VU iteration
export default function(data) {
  const db = new duckdb.DuckDB();
  
  try {
    // Each VU opens its own connection
    db.open(data.dbPath);
    
    // Test 1: Query all users
    const users = db.query("SELECT * FROM users ORDER BY id");
    check(users, {
      'Users query returns results': (r) => r.count > 0,
      'Users query returns expected columns': (r) => r.columns.includes('id') && r.columns.includes('name'),
    });
    
    // Test 2: Query single user
    const userId = Math.floor(Math.random() * 3) + 1; // Random user ID 1-3
    const user = db.querySingle("SELECT * FROM users WHERE id = ?", [userId]);
    check(user, {
      'Single user query returns data': (u) => u !== null,
      'User has required fields': (u) => u && u.id === userId,
    });
    
    // Test 3: Count orders
    const orderCount = db.queryScalar("SELECT COUNT(*) FROM orders");
    check(orderCount, {
      'Order count is valid': (count) => typeof count === 'number' || typeof count === 'bigint',
      'Has orders in database': (count) => Number(count) > 0,
    });
    
    // Test 4: Complex JOIN query
    const userOrders = db.query(`
      SELECT u.name, u.email, o.product, o.amount, o.order_date
      FROM users u
      JOIN orders o ON u.id = o.user_id
      WHERE u.id = ?
      ORDER BY o.order_date DESC
    `, [userId]);
    
    check(userOrders, {
      'JOIN query executes successfully': (r) => r !== null,
      'JOIN query returns expected columns': (r) => 
        r.columns.includes('name') && r.columns.includes('product') && r.columns.includes('amount'),
    });
    
    // Test 5: Insert new order (simulating write operations under load)
    const newOrderId = Date.now() % 1000000; // Simple unique ID
    const products = ['Keyboard', 'Monitor', 'Webcam', 'Headphones', 'Tablet'];
    const randomProduct = products[Math.floor(Math.random() * products.length)];
    const randomAmount = Math.round((Math.random() * 500 + 50) * 100) / 100; // $50-$550
    
    try {
      db.execute(`
        INSERT INTO orders (id, user_id, product, amount) 
        VALUES (?, ?, ?, ?)
      `, [newOrderId, userId, randomProduct, randomAmount]);
      
      // Verify the insert worked
      const insertedOrder = db.querySingle(
        "SELECT * FROM orders WHERE id = ?", [newOrderId]
      );
      
      check(insertedOrder, {
        'Order insertion successful': (order) => order !== null,
        'Inserted order has correct data': (order) => 
          order && order.user_id === userId && order.product === randomProduct,
      });
      
    } catch (error) {
      console.error('Insert operation failed:', error);
    }
    
    // Test 6: Aggregate queries (common in analytics)
    const analytics = db.query(`
      SELECT 
        u.name,
        COUNT(o.id) as order_count,
        COALESCE(SUM(o.amount), 0) as total_spent,
        COALESCE(AVG(o.amount), 0) as avg_order_value
      FROM users u
      LEFT JOIN orders o ON u.id = o.user_id
      GROUP BY u.id, u.name
      ORDER BY total_spent DESC
    `);
    
    check(analytics, {
      'Analytics query returns results': (r) => r.count > 0,
      'Analytics has expected aggregated columns': (r) => 
        r.columns.includes('order_count') && r.columns.includes('total_spent'),
    });
    
    // Test 7: Performance test with larger dataset operations
    const performanceStart = Date.now();
    
    // Create temporary table with generated data
    db.execute(`
      CREATE TEMPORARY TABLE temp_performance_test AS
      SELECT 
        row_number() OVER () as id,
        'user_' || (row_number() OVER ()) as username,
        random() * 1000 as score
      FROM generate_series(1, 1000)
    `);
    
    const perfResults = db.query(`
      SELECT 
        COUNT(*) as total_records,
        AVG(score) as avg_score,
        MIN(score) as min_score,
        MAX(score) as max_score
      FROM temp_performance_test
    `);
    
    const performanceTime = Date.now() - performanceStart;
    
    check(perfResults, {
      'Performance test completes': (r) => r !== null,
      'Performance test processes correct count': (r) => Number(r.rows[0].total_records) === 1000,
      'Performance test completes in reasonable time': () => performanceTime < 5000, // 5 seconds max
    });
    
    console.log(`VU ${__VU}: Processed ${perfResults.rows[0].total_records} records in ${performanceTime}ms`);
    
  } catch (error) {
    console.error(`VU ${__VU} error:`, error);
    check(false, { 'No database errors occurred': () => false });
    
  } finally {
    // Always close the database connection
    try {
      db.close();
    } catch (closeError) {
      console.error('Error closing database:', closeError);
    }
  }
}

// Teardown function runs once after all iterations
export function teardown(data) {
  console.log('Cleaning up after load test...');
  
  const db = new duckdb.DuckDB();
  try {
    db.open(data.dbPath);
    
    // Clean up any test data if using persistent database
    // db.execute("DROP TABLE IF EXISTS temp_test_data");
    
    console.log('Teardown completed successfully');
    
  } catch (error) {
    console.error('Teardown error:', error);
  } finally {
    db.close();
  }
}