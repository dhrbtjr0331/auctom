# Walkthrough: Supplier Agent Strategies & Freemium vs. Premium Quoting

Implemented Phase 1 of the Auctom Product Roadmap: transitioned the supplier's auto-bidding engine from simple randomized numbers into a strategic bidding assistant supporting Freemium (Rules-Based Math Agent) and Premium (Gemini AI Curated Agent) modes.

## Changes Made

### 1. Database Migrations
- Added migration [000004_supplier_strategies.up.sql](file:///Users/dhrbtjr331/Desktop/Projects/auctom/db/migrations/000004_supplier_strategies.up.sql) to add strategy fields to `supplier_profiles`:
  - `target_margin` (Numeric, default 0.20, check 5% - 100%)
  - `utilization_rate` (Numeric, default 0.50, check 0% - 100%)
  - `risk_tolerance` (Numeric, default 0.50, check 0.0 - 1.0)
  - `agent_tier` (Varchar, default 'freemium', check 'freemium'/'premium')
- Added rollback migration [000004_supplier_strategies.down.sql](file:///Users/dhrbtjr331/Desktop/Projects/auctom/db/migrations/000004_supplier_strategies.down.sql).
- Updated [run_services.sh](file:///Users/dhrbtjr331/Desktop/Projects/auctom/run_services.sh) to execute the migration on setup.

### 2. Go API Backend (`go-api`)
- Updated `SupplierProfileResponse` and `RegisterRequest` in [handlers.go](file:///Users/dhrbtjr331/Desktop/Projects/auctom/go-api/internal/handlers/handlers.go) to support the new strategy variables.
- Created `GetSupplierProfile` handler to fetch the authenticated supplier's strategy configuration.
- Created `UpdateSupplierProfile` handler to validate and persist the supplier's strategic parameter adjustments.
- Registered endpoints in [main.go](file:///Users/dhrbtjr331/Desktop/Projects/auctom/go-api/main.go):
  - `GET /api/supplier/profile`
  - `PUT /api/supplier/profile`

### 3. Quote Service (`quote-service`)
- Modified [main.py](file:///Users/dhrbtjr331/Desktop/Projects/auctom/quote-service/main.py):
  - Added direct unit material/labor cost calculator based on part material, size, and type complexity.
  - Implemented Freemium rules-based strategic pricing formula including profit markup, utilization/occupancy scarcity price hike, and risk tolerance offset premium.
  - Implemented Premium Gemini AI Curated Agent using HTTP POST to Google GenAI Developer API with prompt layout and JSON structured output schema configuration. Added fallback safety to Freemium if the key is missing or calls fail.

### 4. Frontend UI (`static/`)
- Updated [index.html](file:///Users/dhrbtjr331/Desktop/Projects/auctom/go-api/static/index.html) to include inputs for Margin, Utilization, Risk Tolerance, and Agent Tier.
- Updated [app.js](file:///Users/dhrbtjr331/Desktop/Projects/auctom/go-api/static/app.js) to dynamically fetch current values on load, update sliders in real-time, and call the PUT endpoint to save values to the database.

---

## Verification & Testing

### Automated Checks
- Verified the Go API compiles successfully: `go build -o /dev/null main.go` works.
- Verified Quote Service compiles successfully: `python3 -m py_compile main.py` works.

### Database Checks
- Verified migration applies successfully:
  ```bash
  psql -d auctom -c "SELECT id, target_margin, utilization_rate, risk_tolerance, agent_tier FROM supplier_profiles LIMIT 1;"
  ```
  Returns:
  ```
                    id                  | target_margin | utilization_rate | risk_tolerance | agent_tier 
  --------------------------------------+---------------+------------------+----------------+------------
   51111111-1111-1111-1111-111111111111 |          0.20 |             0.50 |           0.50 | freemium
  ```
- Verified migration down rollback removes columns clean.
