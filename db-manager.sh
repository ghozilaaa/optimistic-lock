#!/bin/bash

# Database management script for optimistic-lock project

case "$1" in
  start)
    echo "Starting PostgreSQL database..."
    docker compose up -d postgres
    echo "Waiting for database to be ready..."
    sleep 5
    echo "Database started successfully!"
    ;;
  stop)
    echo "Stopping PostgreSQL database..."
    docker compose down
    echo "Database stopped!"
    ;;
  restart)
    echo "Restarting PostgreSQL database..."
    docker compose restart postgres
    echo "Database restarted!"
    ;;
  logs)
    echo "Showing database logs..."
    docker compose logs -f postgres
    ;;
  pgadmin)
    echo "Starting pgAdmin..."
    docker compose up -d pgadmin
    echo "pgAdmin is available at http://localhost:8080"
    echo "Email: admin@example.com, Password: admin"
    ;;
  test)
    echo "Testing database connection..."
    export $(cat .env | xargs)
    go run main.go
    ;;
  psql)
    echo "Connecting to database with psql..."
    export $(cat .env | xargs)
    docker exec -it optimistic_lock_db psql -U $DB_USER -d $DB_NAME
    ;;
  reset)
    echo "Resetting database (removing volumes)..."
    docker compose down -v
    echo "Database reset complete!"
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|logs|pgadmin|test|psql|reset}"
    echo ""
    echo "Commands:"
    echo "  start    - Start the PostgreSQL database"
    echo "  stop     - Stop all services"
    echo "  restart  - Restart the database"
    echo "  logs     - Show database logs"
    echo "  pgadmin  - Start pgAdmin web interface"
    echo "  test     - Test database connection with Go app"
    echo "  psql     - Connect to database with psql client"
    echo "  reset    - Remove all data and reset database"
    exit 1
    ;;
esac