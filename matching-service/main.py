import os
import httpx
import psycopg
import structlog
from fastapi import FastAPI, BackgroundTasks, HTTPException
from pydantic import BaseModel
from psycopg.rows import dict_row

# Environment Variables
DATABASE_URL = os.environ.get(
    "DATABASE_URL",
    "postgres://postgres:postgres@localhost:5432/auctom?sslmode=disable"
)
GO_API_URL = os.environ.get("GO_API_URL", "http://localhost:8080")
QUOTE_SERVICE_URL = os.environ.get("QUOTE_SERVICE_URL", "http://localhost:9092")

# Structlog Setup
structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.add_log_level,
        structlog.processors.dict_tracebacks,
        structlog.processors.JSONRenderer(),
    ]
)
logger = structlog.get_logger()

app = FastAPI(title="Matching Service")

class RFQPayload(BaseModel):
    rfq_id: str

def send_status_callback(rfq_id: str, status: str):
    url = f"{GO_API_URL}/api/webhooks/status-callback"
    payload = {"rfq_id": rfq_id, "status": status}
    logger.info("Sending status callback to Go API", url=url, payload=payload)
    try:
        with httpx.Client(timeout=10.0) as client:
            resp = client.post(url, json=payload)
            resp.raise_for_status()
            logger.info("Status callback succeeded", url=url, status_code=resp.status_code)
    except Exception as e:
        logger.error("Failed to send status callback to Go API", url=url, error=str(e))

def trigger_quote_service(rfq_id: str):
    url = f"{QUOTE_SERVICE_URL}/webhook/rfq-matched"
    payload = {"rfq_id": rfq_id}
    logger.info("Triggering Quote Service webhook", url=url, payload=payload)
    try:
        with httpx.Client(timeout=10.0) as client:
            resp = client.post(url, json=payload)
            resp.raise_for_status()
            logger.info("Triggering Quote Service succeeded", url=url, status_code=resp.status_code)
    except Exception as e:
        logger.error("Failed to trigger Quote Service webhook", url=url, error=str(e))

def process_rfq_created(rfq_id: str):
    logger.info("Processing RFQ creation", rfq_id=rfq_id)
    try:
        with psycopg.connect(DATABASE_URL) as conn:
            with conn.cursor(row_factory=dict_row) as cur:
                # Fetch RFQ details
                cur.execute("SELECT required_capability FROM rfqs WHERE id = %s", (rfq_id,))
                rfq = cur.fetchone()
                if not rfq:
                    logger.error("RFQ not found in database", rfq_id=rfq_id)
                    return
                
                req_cap = rfq["required_capability"]
                logger.info("Found RFQ capability requirement", rfq_id=rfq_id, required_capability=req_cap)

                # Find matching supplier profiles
                cur.execute("SELECT id FROM supplier_profiles WHERE %s = ANY(capabilities)", (req_cap,))
                suppliers = cur.fetchall()
                logger.info("Found matching suppliers", rfq_id=rfq_id, count=len(suppliers))

                # Insert matches into rfq_matches
                for sp in suppliers:
                    cur.execute(
                        "INSERT INTO rfq_matches (rfq_id, supplier_id) VALUES (%s, %s) ON CONFLICT DO NOTHING",
                        (rfq_id, sp["id"])
                    )
                
                # Update RFQ status to 'matched'
                cur.execute("UPDATE rfqs SET status = 'matched' WHERE id = %s", (rfq_id,))
                logger.info("Updated RFQ status to matched in database", rfq_id=rfq_id)
    except Exception as e:
        logger.exception("Database transaction failed in Matching Service", rfq_id=rfq_id, error=str(e))
        return

    # Outside the DB transaction block to avoid race conditions with transaction commits
    send_status_callback(rfq_id, "matched")
    trigger_quote_service(rfq_id)

@app.post("/webhook/rfq-created")
async def rfq_created(payload: RFQPayload, background_tasks: BackgroundTasks):
    logger.info("Received rfq-created webhook", rfq_id=payload.rfq_id)
    background_tasks.add_task(process_rfq_created, payload.rfq_id)
    return {"status": "processing"}

@app.get("/health")
def health():
    return {"status": "ok"}
