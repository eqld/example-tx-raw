package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	dbUser     = "exampleuser"
	dbPassword = "examplepassword"
	dbName     = "exampledb"
	dbHost     = "localhost"
	dbPort     = "54320"
	tableName  = "items"
)

// Tx is a type based on sql.Tx that provides a Raw() method using reflection.
// This demonstrates the workaround currently needed to access the underlying
// driver connection from within a transaction context.
//
// IMPORTANT: This reflection-based approach is fragile and depends on the
// internal structure of sql.Tx, which could change between Go versions.
// An official Tx.Raw() method in the standard library would eliminate
// the need for this unsafe workaround.
type Tx sql.Tx

// Raw executes the provided function with access to the underlying driver connection.
// This method uses reflection to access unexported fields of sql.Tx, which is
// necessary because sql.Tx doesn't provide a Raw() method like sql.Conn does.
//
// The reflection process:
// 1. Access sql.Tx.dc (driverConn) field
// 2. Extract dc.ci (driver.Conn interface)
// 3. Execute the callback with the driver connection
//
// This approach is fragile because:
// - It depends on internal Go standard library structure
// - Field names and types could change between Go versions
// - It bypasses Go's type safety and encapsulation
func (tx *Tx) Raw(f func(driverConn any) error) (err error) {
	// Use reflection to access `tx.dc` (`driverConn`).
	txValue := reflect.ValueOf((*sql.Tx)(tx)).Elem()

	dcField := txValue.FieldByName("dc")
	if !dcField.IsValid() {
		return fmt.Errorf("cannot access dc field from transaction")
	}

	// Make the field accessible and get the `driverConn` pointer.
	dcField = reflect.NewAt(dcField.Type(), dcField.Addr().UnsafePointer()).Elem()
	dc := dcField.Interface()
	dcValue := reflect.ValueOf(dc).Elem()

	// Access `dc.ci` (`driver.Conn` interface).
	ciField := dcValue.FieldByName("ci")
	if !ciField.IsValid() {
		return fmt.Errorf("cannot access ci field from `driverConn`")
	}

	// Make the field accessible and get the underlying driver connection.
	ciField = reflect.NewAt(ciField.Type(), ciField.Addr().UnsafePointer()).Elem()
	ci := ciField.Interface()

	return f(ci)
}

func main() {
	log.Println("=== Go sql.Tx Raw Connection Access Example ===")
	log.Println("This example demonstrates the need for an official Tx.Raw() method")
	log.Println("in Go's database/sql package by showing pgx.CopyFrom usage scenarios.")
	log.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Establish database connection
	db, err := dbConnect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run all three demonstration scenarios
	demonstrateNoTransactionCopyFrom(ctx, db)
	demonstrateTransactionCommitCopyFrom(ctx, db)
	demonstrateTransactionRollbackCopyFrom(ctx, db)

	log.Println("\n=== Example Finished ===")
	log.Println("Key observations:")
	log.Println("1. Non-transactional CopyFrom works cleanly with sql.Conn.Raw()")
	log.Println("2. Transactional CopyFrom requires fragile reflection workarounds")
	log.Println("3. An official Tx.Raw() method would solve this problem safely")
	log.Println()
	log.Println("This example provides justification for adding Tx.Raw() to database/sql")
}

// demonstrateNoTransactionCopyFrom shows how pgx.CopyFrom works perfectly
// in a non-transactional context using the official sql.Conn.Raw() method.
//
// This scenario works cleanly because sql.Conn provides a Raw() method
// that allows safe access to the underlying driver connection.
func demonstrateNoTransactionCopyFrom(ctx context.Context, db *sql.DB) {
	log.Println("--- Scenario 1: CopyFrom WITHOUT transaction ---")
	log.Println("Uses sql.Conn.Raw() - the official, safe way to access driver connection")

	if err := clearTable(ctx, db); err != nil {
		log.Fatalf("Failed to clear table: %v", err)
	}

	// Generate sample data for bulk insertion
	sampleData := generateSampleData(10, "NoTx")
	log.Printf("Generated %d rows for non-transactional insertion", len(sampleData))

	// Get a connection from the pool
	sqlDBConn, err := db.Conn(ctx)
	if err != nil {
		log.Fatalf("db.Conn failed: %v", err)
	}
	defer sqlDBConn.Close()

	// Use the official Raw() method - this is the clean, supported approach
	err = sqlDBConn.Raw(func(driverConn any) error {
		return performCopyFrom(ctx, driverConn, sampleData, "non-transactional")
	})
	if err != nil {
		log.Fatalf("sqlDBConn.Raw failed: %v", err)
	}

	// Verify the results
	rowCount, err := countRows(ctx, db)
	if err != nil {
		log.Fatalf("Failed to count rows (no-tx): %v", err)
	}

	log.Printf("✓ Result: %d rows inserted (Expected: %d)", rowCount, len(sampleData))
	if rowCount != len(sampleData) {
		log.Printf("✗ ERROR: Row count mismatch for non-transactional CopyFrom!")
	}
	log.Println()
}

// demonstrateTransactionCommitCopyFrom shows how pgx.CopyFrom can be used
// within a transaction context, but requires reflection-based workarounds
// because sql.Tx doesn't provide a Raw() method.
//
// This scenario demonstrates the problem: we need unsafe reflection to
// access the driver connection from within a transaction.
func demonstrateTransactionCommitCopyFrom(ctx context.Context, db *sql.DB) {
	log.Println("--- Scenario 2: CopyFrom WITH transaction (COMMIT) ---")
	log.Println("Uses reflection-based Tx.Raw() - demonstrates the current workaround")

	if err := clearTable(ctx, db); err != nil {
		log.Fatalf("Failed to clear table: %v", err)
	}

	// Generate sample data for transactional insertion
	sampleData := generateSampleData(15, "TxCommit")
	log.Printf("Generated %d rows for transactional insertion (commit)", len(sampleData))

	// Begin transaction
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to begin transaction (commit scenario): %v", err)
	}

	// Wrap sql.Tx to add our reflection-based Raw() method
	tx := (*Tx)(sqlTx)

	// Use our reflection-based Raw() method - this is the problematic workaround
	log.Println("⚠️  Using reflection to access transaction's driver connection...")
	err = tx.Raw(func(driverConn any) error {
		return performCopyFrom(ctx, driverConn, sampleData, "transactional (commit)")
	})

	if err != nil {
		log.Printf("✗ CopyFrom failed, rolling back: %v", err)
		if rollbackErr := sqlTx.Rollback(); rollbackErr != nil {
			log.Printf("✗ Rollback also failed: %v", rollbackErr)
		}
		return
	}

	// Commit the transaction
	if err = sqlTx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}
	log.Println("✓ Transaction committed successfully")

	// Verify the results
	rowCount, err := countRows(ctx, db)
	if err != nil {
		log.Fatalf("Failed to count rows (tx-commit): %v", err)
	}

	log.Printf("✓ Result: %d rows persisted after commit (Expected: %d)", rowCount, len(sampleData))
	if rowCount != len(sampleData) {
		log.Printf("✗ ERROR: Row count mismatch for transactional CopyFrom!")
	}
	log.Println()
}

// demonstrateTransactionRollbackCopyFrom shows how pgx.CopyFrom works within
// a transaction that gets rolled back, again requiring reflection workarounds.
//
// This scenario proves that the transactional semantics work correctly
// even with the reflection-based approach, but highlights the fragility.
func demonstrateTransactionRollbackCopyFrom(ctx context.Context, db *sql.DB) {
	log.Println("--- Scenario 3: CopyFrom WITH transaction (ROLLBACK) ---")
	log.Println("Uses reflection-based Tx.Raw() - demonstrates transaction rollback")

	if err := clearTable(ctx, db); err != nil {
		log.Fatalf("Failed to clear table: %v", err)
	}

	// Generate sample data for transactional insertion that will be rolled back
	sampleData := generateSampleData(20, "TxRollback")
	log.Printf("Generated %d rows for transactional insertion (rollback)", len(sampleData))

	// Begin transaction
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to begin transaction (rollback scenario): %v", err)
	}

	// Wrap sql.Tx to add our reflection-based Raw() method
	tx := (*Tx)(sqlTx)

	// Use our reflection-based Raw() method
	log.Println("⚠️  Using reflection to access transaction's driver connection...")
	err = tx.Raw(func(driverConn any) error {
		return performCopyFrom(ctx, driverConn, sampleData, "transactional (rollback)")
	})

	if err != nil {
		log.Printf("✗ CopyFrom failed: %v", err)
		if rollbackErr := sqlTx.Rollback(); rollbackErr != nil {
			log.Printf("✗ Rollback also failed: %v", rollbackErr)
		}
		return
	}

	// Intentionally rollback the transaction to demonstrate transactional semantics
	if err = sqlTx.Rollback(); err != nil {
		log.Fatalf("Failed to rollback transaction: %v", err)
	}
	log.Println("✓ Transaction rolled back successfully")

	// Verify that no data was persisted
	rowCount, err := countRows(ctx, db)
	if err != nil {
		log.Fatalf("Failed to count rows (tx-rollback): %v", err)
	}

	log.Printf("✓ Result: %d rows persisted after rollback (Expected: 0)", rowCount)
	if rowCount != 0 {
		log.Printf("✗ ERROR: Data was persisted despite rollback!")
	} else {
		log.Println("✓ Rollback worked correctly - no data persisted")
	}
	log.Println()
}

// performCopyFrom encapsulates the common logic for executing pgx.CopyFrom
// with proper error handling and logging.
//
// The function expects a driver connection (should be *stdlib.Conn for pgx)
// and performs the bulk insertion using pgx's efficient CopyFrom method.
func performCopyFrom(ctx context.Context, driverConn any, data [][]any, scenario string) error {
	// Cast the driver connection to pgx's stdlib.Conn
	stdlibConn, ok := driverConn.(*stdlib.Conn)
	if !ok {
		return fmt.Errorf("driverConn is not *stdlib.Conn, got %T", driverConn)
	}

	// Get the underlying pgx.Conn which provides the CopyFrom method
	pgxConn := stdlibConn.Conn()

	// Perform the bulk insertion using pgx's high-performance CopyFrom
	// This is significantly faster than individual INSERT statements
	copyCount, err := pgxConn.CopyFrom(
		ctx,
		pgx.Identifier{tableName},
		[]string{"name", "data"}, // Column names must match table schema
		pgx.CopyFromRows(data),
	)
	if err != nil {
		return fmt.Errorf("pgxConn.CopyFrom failed: %w", err)
	}

	log.Printf("✓ Successfully inserted %d rows using CopyFrom (%s)", copyCount, scenario)
	return nil
}

// dbConnect establishes a connection to the PostgreSQL database using pgx driver.
// The connection string is configured for the Docker container setup.
func dbConnect(ctx context.Context) (*sql.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open failed: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db.PingContext failed: %w", err)
	}

	log.Println("✓ Successfully connected to PostgreSQL")
	return db, nil
}

// generateSampleData creates a slice of sample data for CopyFrom operations.
// Each row contains a name and data field with the specified prefix.
//
// The data format matches the table schema: (name VARCHAR, data TEXT)
func generateSampleData(numRows int, prefix string) [][]any {
	data := make([][]any, numRows)
	for i := 0; i < numRows; i++ {
		data[i] = []any{
			fmt.Sprintf("%s Name %d", prefix, i+1),
			fmt.Sprintf("%s Data %d", prefix, i+1),
		}
	}
	return data
}

// countRows returns the number of rows in the items table.
// It accepts any querier interface to work with both *sql.DB and *sql.Tx.
func countRows(ctx context.Context, querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (int, error) {
	var count int
	err := querier.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("QueryRowContext for COUNT failed: %w", err)
	}
	return count, nil
}

// clearTable removes all rows from the items table to ensure clean test state.
func clearTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("DELETE FROM %s failed: %w", tableName, err)
	}
	log.Printf("✓ Table %s cleared", tableName)
	return nil
}
