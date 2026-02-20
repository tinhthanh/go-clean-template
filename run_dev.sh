#!/bin/bash
set -e

# Create .env if it doesn't exist
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
fi

# detailed check to ensure .env is valid and has content before exporting
if [ -s .env ]; then
  echo "Exporting environment variables from .env..."
  set -a # automatically export all variables
  source .env
  set +a
else 
  echo "Error: .env file is empty or missing."
  exit 1
fi

# Start Docker dependencies (without tailing logs)
echo "Starting Docker dependencies..."
docker compose -f docker-compose.yml up --build -d db rabbitmq nats

# Wait for DB to be ready (optional, but good practice)
echo "Waiting for services to spin up..."
sleep 5

# Run the application
echo "Starting Go application..."
# Check if PG_URL is set (sanity check)
if [ -z "$PG_URL" ]; then
    echo "Error: PG_URL is not set. Please check .env file."
    exit 1
fi

# Using tags 'migrate' to run migrations on startup as per Makefile 'run' target
go run -tags migrate ./cmd/app
