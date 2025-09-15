# Makefile for optimistic-lock project

.PHONY: help test test-verbose test-coverage clean build run db-start db-stop db-restart deps

# Default target
help:
	@echo "Available commands:"
	@echo "  make test          - Run all tests"
	@echo "  make test-verbose  - Run tests with verbose output"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make build         - Build the application"
	@echo "  make run           - Run the application"
	@echo "  make db-start      - Start database"
	@echo "  make db-stop       - Stop database"
	@echo "  make db-restart    - Restart database"
	@echo "  make deps          - Download dependencies"
	@echo "  make clean         - Clean build artifacts"

# Test targets
test:
	@echo "Running tests..."
	go test ./test/

test-verbose:
	@echo "Running tests with verbose output..."
	go test -v ./test/

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover ./test/
	go test -coverprofile=coverage.out ./test/
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build and run
build:
	@echo "Building application..."
	go build -o bin/optimistic-lock .

run: build
	@echo "Running application..."
	./bin/optimistic-lock

# Database management
db-start:
	@echo "Starting database..."
	./db-manager.sh start

db-stop:
	@echo "Stopping database..."
	./db-manager.sh stop

db-restart:
	@echo "Restarting database..."
	./db-manager.sh stop
	./db-manager.sh start

# Dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html