# Auctom | Event-Driven Manufacturing Procurement Platform

Auctom is a full-stack, event-driven multi-service RFQ (Request for Quote) platform designed to automate and streamline supply chain procurement for manufacturing.

---

## 📌 Motivation & Objective

Traditional manufacturing procurement is a highly manual, slow, and fragmented process:
- **Opaque Quoting**: Suppliers manually calculate costs, leading to long turnaround times and inconsistent pricing.
- **Manual Evaluation**: Buyers must review multiple bids via emails/spreadsheets, struggling to balance conflicting priorities like cost, lead time, and compliance risk.
- **Workflow Bottlenecks**: Awarding contracts and notifying suppliers involves significant administrative overhead.

**Auctom** automates the entire lifecycle:
1. A **Buyer** posts a parts RFQ with specifications and multi-criteria priority weightings.
2. The **Matching Service** instantly identifies eligible suppliers based on capability requirements.
3. The **Quote Service** auto-generates optimized quote suggestions using strategic pricing engines.
4. The **Recommender Service** normalizes and ranks the bids according to buyer-defined priority weights.
5. The **Auto-Award Engine** awards the contract instantly if target thresholds are met, or highlights the top bid for one-click approval.

---

## 🛠️ Architecture Design

Auctom utilizes an event-driven microservices architecture sharing a single PostgreSQL database. Services coordinate asynchronously via webhooks, notifying the Go API server to broadcast real-time events to client dashboards via Server-Sent Events (SSE).

```
  [Buyer UI] ──(1. Create RFQ)──> [Go API Server :8080]
          │                         │ (2. Webhook: rfq.created)
          │                         ▼
          │                   [Matching Service :9091]
          │                         │ (3. Webhook: rfq.matched)
          │                         ▼
          │                   [Quote Service :9092]
          │                         │ (4. Webhook: quotes.generated)
          │                         ▼
          │                   [Recommender Service :9093]
          │                         │ (5. Callback: ranked)
          │                         ▼
  [Buyer UI] <──(6. SSE Updates)─── [Go API Server :8080]
```

### **Tech Stack**
* **Backend Gateway (Go)**: Go 1.25, `go-chi/chi/v5` Router, JWT Auth, `lib/pq` Driver, SSE Event Broker.
* **Microservices (Python)**: FastAPI, `psycopg` (v3), `httpx`, `structlog`, `numpy` (for matrix math and normalization).
* **Database**: PostgreSQL 15.
* **Frontend**: Vanilla ES6 JS, HTML5, CSS3 served directly by the Go backend.

---

## 🚀 Key Features

### 1. **Buyer RFQ & Procurement Dashboard**
- **Detailed Part Specs**: Post RFQs specifying required capability, part dimensions, material requirements, and custom notes.
- **Priority-Weight Optimization**: Adjust sliders for Cost, Lead Time, and Risk. Sliders link dynamically in the UI to ensure their sum always equals 100%.
- **Live SSE Progress Timeline**: Watch the RFQ transition live from `created` -> `matching` -> `quotes_generated` -> `ranked` -> `awarded`.
- **Private Selective Notifications**: Toasts and updates are cryptographically authenticated via JWT query parameters and restricted only to associated buyer and supplier accounts.

### 2. **Buyer Auto-Award Rules & Guardrails**
- **Disabled**: Standard flow. The buyer manually reviews the ranked recommendations and awards the contract.
- **Instant Auto-Award**: If the top-ranked recommendation meets or exceeds the **Min Score Threshold** and falls within the **Target Budget**, the contract is awarded instantly without human intervention.
- **One-Click Approve**: Highlights the top recommendation with a glowing green badge and button if it meets the budget and score criteria.

### 3. **Supplier Strategic Quoting Engine**
Suppliers can configure their profile settings to drive quote generation:
- **Auto-Bid Toggle**: Automatically publishes generated quotes as active bids.
- **Margin Slider**: Mark up profit margins from 5% to 100%.
- **Utilization Slider**: Adjusts prices based on machine occupancy (higher utilization increases scarcity pricing).
- **Risk Tolerance Slider**: Accounts for complex geometry or unfamiliar specifications.
- **Quoting Autonomy Tiers**:
  - **Freemium (Rules-Based Math Agent)**: Computes direct material and labor costs from volume metrics, applying margin markup, utilization scarcity factors, and risk premiums.
  - **Premium (Gemini AI Agent)**: Integrates with Google's GenAI developer API. It submits RFQ details to a Gemini model to dynamically negotiate and output structured quote recommendations.

---

## 📁 Repository Structure

```
├── db/
│   └── migrations/                 # PostgreSQL schema and seed migrations
├── go-api/
│   ├── internal/                   # JWT Auth, SSE Broker, and REST handlers
│   ├── static/                     # Dark-mode dashboard SPA (HTML/CSS/JS)
│   └── main.go                     # Gateway server entry point
├── matching-service/               # Python capability matching microservice
├── quote-service/                  # Python strategic quote generation service
├── recommender-service/            # Python normalization and scoring service
├── docker-compose.yml              # Container orchestration setup
├── run_services.sh                 # Local bash starter script
└── README.md                       # Documentation
```

---

## ⚙️ How to Run & Verify

### **Prerequisites**
- PostgreSQL running locally on port 5432, or Docker installed.
- Go (1.22+) and Python (3.12+) installed.

### **Local Setup (Recommended)**
To start the database migrations, set up Python virtual environments, install dependencies, and run all services:
```bash
# Make starter script executable
chmod +x run_services.sh

# Run services
./run_services.sh
```
Once initialized, access the dashboard at: **`http://localhost:8080/`**.

### **Docker Compose Setup**
To run everything containerized (mapping database to port `5435` to avoid local host port conflicts):
```bash
docker-compose up --build
```

### **Integration Verification**
Run the automated end-to-end webhook validation script:
```bash
python3 go-api/static/../../*/*/scratch/test_pipeline.py
```
*(Or specify the absolute path to `test_pipeline.py` inside the App Data brain workspace directory).*

---

## 🔑 Demo Accounts (Pre-Seeded)
Use the following credentials to log in:
- **Buyer**: `buyer@auctom.com` / `password123`
- **Supplier 1 (Apex CNC Machining - Auto-Bid)**: `supplier1@auctom.com` / `password123`
- **Supplier 2 (Vertex 3D & Molding - Manual)**: `supplier2@auctom.com` / `password123`
