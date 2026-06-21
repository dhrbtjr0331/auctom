#!/bin/bash

# Kill all background jobs on exit
trap 'kill $(jobs -p)' EXIT

echo "=================================================="
echo "          Auctom Platform Local Starter           "
echo "=================================================="

# Export common env variables
export DATABASE_URL="postgres://localhost:5432/auctom?sslmode=disable"
export GO_API_URL="http://localhost:8080"
export MATCHING_SERVICE_URL="http://localhost:9091"
export QUOTE_SERVICE_URL="http://localhost:9092"
export RECOMMENDER_SERVICE_URL="http://localhost:9093"

# 1. Initialize and Seed database
echo "[1/5] Initializing Database 'auctom'..."
createdb auctom 2>/dev/null || echo "Database might already exist."
psql -d auctom -f db/migrations/000001_init_schema.up.sql
psql -d auctom -f db/migrations/000002_seed_data.up.sql
echo "Database schema and seed data applied!"
echo ""

# 2. Run Matching Service
echo "[2/5] Starting Matching Service on port 9091..."
cd matching-service
if [ ! -d "venv" ]; then
    python3 -m venv venv
fi
source venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --port 9091 &
MATCHING_PID=$!
deactivate
cd ..
echo "Matching Service running (PID: $MATCHING_PID)"
echo ""

# 3. Run Quote Service
echo "[3/5] Starting Quote Service on port 9092..."
cd quote-service
if [ ! -d "venv" ]; then
    python3 -m venv venv
fi
source venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --port 9092 &
QUOTE_PID=$!
deactivate
cd ..
echo "Quote Service running (PID: $QUOTE_PID)"
echo ""

# 4. Run Recommender Service
echo "[4/5] Starting Recommender Service on port 9093..."
cd recommender-service
if [ ! -d "venv" ]; then
    python3 -m venv venv
fi
source venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --port 9093 &
RECOMMENDER_PID=$!
deactivate
cd ..
echo "Recommender Service running (PID: $RECOMMENDER_PID)"
echo ""

# Wait a second for microservices to bind to ports
sleep 2

# 5. Run Go API Server
echo "[5/5] Starting Go API Server on port 8080..."
echo "Open http://localhost:8080/ in your browser once the server starts."
echo ""
cd go-api
go run main.go
