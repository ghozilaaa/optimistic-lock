# Optimistic Lock Database Setup

This project uses Docker Compose to manage a PostgreSQL database for testing optimistic locking with GORM.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.23.2 or later

## Quick Start

### 1. Start the Database

```bash
# Start PostgreSQL database
./db-manager.sh start

# Or manually with docker compose
docker compose up -d postgres
```

### 2. Test the Connection

```bash
# Test database connection with the Go application
./db-manager.sh test

# Or manually
export $(cat .env | xargs) && go run main.go
```

### 3. Access pgAdmin (Optional)

```bash
# Start pgAdmin web interface
./db-manager.sh pgadmin
```

Then visit http://localhost:8080
- Email: `admin@example.com`
- Password: `admin`

## Database Management Commands

The `db-manager.sh` script provides convenient commands:

```bash
./db-manager.sh start     # Start PostgreSQL database
./db-manager.sh stop      # Stop all services
./db-manager.sh restart   # Restart the database
./db-manager.sh logs      # Show database logs
./db-manager.sh pgadmin   # Start pgAdmin web interface
./db-manager.sh test      # Test database connection with Go app
./db-manager.sh psql      # Connect to database with psql client
./db-manager.sh reset     # Remove all data and reset database
```

## Configuration

Database configuration is stored in `.env`:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=optimistic_lock
DB_SSLMODE=disable
```

## Docker Compose Services

- **postgres**: PostgreSQL 15 database on port 5432
- **pgadmin**: pgAdmin web interface on port 8080

## Database Schema

The application uses GORM to auto-migrate the `Balance` model:

```go
type Balance struct {
    ID      uint  `gorm:"primaryKey"`
    Amount  int64 // balance amount
    Version int   `gorm:"version"` // optimistic locking version
}
```

## Troubleshooting

### Database Connection Issues

1. Ensure Docker is running
2. Check if the database container is up: `docker ps`
3. Verify environment variables: `cat .env`
4. Check database logs: `./db-manager.sh logs`

### Reset Database

If you need to start fresh:

```bash
./db-manager.sh reset
./db-manager.sh start
```

This will remove all data and recreate the database.

## Development Workflow

1. Start the database: `./db-manager.sh start`
2. Run your Go application: `go run main.go`
3. Run tests: `go test ./test/`
4. View logs if needed: `./db-manager.sh logs`
5. Stop when done: `./db-manager.sh stop`