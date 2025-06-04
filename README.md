# Go `sql.Tx` Raw Connection Access - Example Use Case

This example application demonstrates a compelling use case for adding a `Raw()` method to Go's `sql.Tx` type. It showcases the challenges developers face when trying to use driver-specific functionality (like PostgreSQL's `COPY FROM` via the `pgx` driver) within database transactions.

## Table of Contents

- [The Problem](#the-problem)
- [Current Workaround: Reflection](#current-workaround-reflection)
- [Proposed Solution](#proposed-solution)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [How to Run](#how-to-run)
- [What This Example Demonstrates](#what-this-example-demonstrates)
- [Expected Output](#expected-output)
- [Database Schema](#database-schema)
- [Technical Implementation](#technical-implementation)
- [Troubleshooting](#troubleshooting)

## The Problem

Go's `database/sql` package provides excellent abstractions for database operations, including the `sql.Tx` type for managing transactions. However, it lacks a crucial feature that `sql.Conn` provides: direct access to the underlying driver connection.

While `sql.Conn` offers a `Raw()` method:
```go
// From database/sql/sql.go
func (c *Conn) Raw(f func(driverConn any) error) error
```

The `sql.Tx` type has no equivalent method, creating a significant limitation when:

1. **Driver-specific functionality** is needed within a transaction context
2. **Performance-critical operations** (like bulk inserts) require direct driver access
3. **Advanced features** not exposed through standard `database/sql` interfaces are required

### Real-World Example: PostgreSQL COPY FROM

The `pgx` driver provides `CopyFrom`, a highly efficient method for bulk data insertion that can be **10-100x faster** than individual `INSERT` statements. However, using this within a transaction currently requires unsafe workarounds.

## Current Workaround: Reflection

To access the underlying driver connection from `sql.Tx`, developers must resort to reflection-based hacks that:

- **Break encapsulation** by accessing unexported fields
- **Are fragile** and can break between Go versions
- **Bypass type safety** mechanisms
- **Add complexity** and maintenance burden

This example includes a working implementation of such a workaround, demonstrating both its necessity and its problems.

## Proposed Solution

Add a `Raw()` method to `sql.Tx` similar to the existing `sql.Conn.Raw()`:

```go
// Proposed addition to database/sql
func (tx *Tx) Raw(f func(driverConn any) error) error {
    // Safe, official implementation
}
```

This would provide a **clean, safe, and official** way to access driver-specific functionality within transactions.

## Project Structure

```
.
├── main.go              # Main application demonstrating the use cases
├── README.md            # This documentation
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── Makefile             # Build and run automation
├── docker-compose.yml   # PostgreSQL container configuration
├── init.sql             # Database schema initialization
├── .gitignore           # Git ignore rules
└── LICENSE              # Project license
```

## Prerequisites

- **Go 1.24+** (as specified in go.mod)
- **Docker** and **Docker Compose** for PostgreSQL container
- **Make** (optional, for convenient commands)

### Installation Check

Verify your setup:
```bash
go version          # Should show Go 1.24+
docker --version    # Should show Docker installation
docker-compose --version  # Should show Docker Compose
```

## How to Run

### Quick Start

```bash
# Clone and navigate to the project
git clone github.com/eqld/example-tx-raw
cd example-tx-raw

# Run the complete example (starts DB, runs app, cleans up)
make run
```

### Manual Steps

If you prefer to run steps manually:

```bash
# 1. Start PostgreSQL container
make up

# 2. Run the Go application
go run main.go

# 3. Clean up (stop and remove container)
make down
```

### Alternative Commands

```bash
# View PostgreSQL logs
make logs

# Build executable
make build

# Clean all Docker resources
make clean
```

## What This Example Demonstrates

The application runs three scenarios to illustrate the problem and solution:

### 1. **Non-Transactional CopyFrom** ✅
- Uses `sql.Conn.Raw()` - the **official, safe approach**
- Demonstrates how it should work when not in a transaction
- Shows clean, straightforward code

### 2. **Transactional CopyFrom (Commit)** ⚠️
- Uses **reflection-based workaround** to access driver connection
- Successfully inserts data and commits the transaction
- Highlights the complexity and fragility of current solutions

### 3. **Transactional CopyFrom (Rollback)** ⚠️
- Uses the same **reflection-based workaround**
- Inserts data then rolls back to verify transactional semantics
- Proves that the workaround maintains transaction integrity

## Expected Output

When you run the example, you should see output similar to:

```
=== Go sql.Tx Raw Connection Access Example ===
This example demonstrates the need for an official Tx.Raw() method
in Go's database/sql package by showing pgx.CopyFrom usage scenarios.

✓ Successfully connected to PostgreSQL
✓ Table items cleared

--- Scenario 1: CopyFrom WITHOUT transaction ---
Uses sql.Conn.Raw() - the official, safe way to access driver connection
Generated 10 rows for non-transactional insertion
✓ Successfully inserted 10 rows using CopyFrom (non-transactional)
✓ Result: 10 rows inserted (Expected: 10)

--- Scenario 2: CopyFrom WITH transaction (COMMIT) ---
Uses reflection-based Tx.Raw() - demonstrates the current workaround
✓ Table items cleared
Generated 15 rows for transactional insertion (commit)
⚠️  Using reflection to access transaction's driver connection...
✓ Successfully inserted 15 rows using CopyFrom (transactional (commit))
✓ Transaction committed successfully
✓ Result: 15 rows persisted after commit (Expected: 15)

--- Scenario 3: CopyFrom WITH transaction (ROLLBACK) ---
Uses reflection-based Tx.Raw() - demonstrates transaction rollback
✓ Table items cleared
Generated 20 rows for transactional insertion (rollback)
⚠️  Using reflection to access transaction's driver connection...
✓ Successfully inserted 20 rows using CopyFrom (transactional (rollback))
✓ Transaction rolled back successfully
✓ Result: 0 rows persisted after rollback (Expected: 0)
✓ Rollback worked correctly - no data persisted

=== Example Finished ===
Key observations:
1. Non-transactional CopyFrom works cleanly with sql.Conn.Raw()
2. Transactional CopyFrom requires fragile reflection workarounds
3. An official Tx.Raw() method would solve this problem safely
```

## Database Schema

The example uses a simple PostgreSQL table:

```sql
CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    data TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

**Connection Details:**
- Host: `localhost:54320` (mapped from container's 5432)
- Database: `exampledb`
- User: `exampleuser`
- Password: `examplepassword`

## Technical Implementation

### Custom Tx Type with Reflection

The example implements a custom `Tx` type that wraps `sql.Tx` and adds a `Raw()` method:

```go
type Tx sql.Tx

func (tx *Tx) Raw(f func(driverConn any) error) error {
    // Uses reflection to access unexported fields:
    // 1. sql.Tx.dc (driverConn)
    // 2. driverConn.ci (driver.Conn interface)
    // 3. Executes callback with driver connection
}
```

### Performance Benefits

`pgx.CopyFrom` provides significant performance improvements:
- **Standard INSERTs**: ~1,000-10,000 rows/second
- **CopyFrom**: ~100,000-1,000,000 rows/second
- **Use cases**: Data migrations, bulk imports, ETL processes

### Safety Concerns

The reflection approach:
- Accesses `sql.Tx.dc` and `driverConn.ci` unexported fields
- Depends on internal Go standard library structure
- Could break with Go version updates
- Bypasses intended encapsulation

## Troubleshooting

### Common Issues

**PostgreSQL Connection Failed**
```bash
# Check if container is running
docker ps | grep postgres_tx_raw_example

# Check container logs
make logs

# Restart container
make down && make up
```

**Port 54320 Already in Use**
```bash
# Find process using the port
lsof -i :54320

# Kill the process or change port in docker-compose.yml
```

**Go Module Issues**
```bash
# Clean and rebuild module cache
go clean -modcache
go mod download
go mod tidy
```

**Permission Errors**
```bash
# Ensure Docker daemon is running
sudo systemctl start docker  # Linux
# or restart Docker Desktop    # macOS/Windows
```

### Verification Steps

1. **Database Connection**: The app will fail fast if PostgreSQL isn't accessible
2. **Table Creation**: Check `init.sql` is properly mounted and executed
3. **Data Insertion**: Each scenario reports success/failure with row counts
4. **Transaction Semantics**: Rollback scenario should show 0 persisted rows

## Contributing

This example serves as supporting material for a potential Go standard library enhancement. The goal is to demonstrate real-world need for `sql.Tx.Raw()` functionality.

**Key Points for Go Proposal:**
- Demonstrates concrete use case (bulk inserts in transactions)
- Shows current workaround fragility
- Proves transaction semantics work with driver access
- Provides performance justification
- Maintains backward compatibility

---

*This example provides concrete justification for adding `Tx.Raw()` to Go's `database/sql` package, enabling safe access to driver-specific functionality within transaction contexts.*
