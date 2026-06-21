import os
import hashlib
import httpx
import psycopg
import structlog
import numpy as np
from fastapi import FastAPI, BackgroundTasks, HTTPException
from pydantic import BaseModel
from psycopg.rows import dict_row

# Environment Variables
DATABASE_URL = os.environ.get(
    "DATABASE_URL",
    "postgres://postgres:postgres@localhost:5432/auctom?sslmode=disable"
)
GO_API_URL = os.environ.get("GO_API_URL", "http://localhost:8080")
RECOMMENDER_SERVICE_URL = os.environ.get("RECOMMENDER_SERVICE_URL", "http://localhost:9093")

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

app = FastAPI(title="Quote Service")

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

def trigger_recommender_service(rfq_id: str):
    url = f"{RECOMMENDER_SERVICE_URL}/webhook/quotes-generated"
    payload = {"rfq_id": rfq_id}
    logger.info("Triggering Recommender Service webhook", url=url, payload=payload)
    try:
        with httpx.Client(timeout=10.0) as client:
            resp = client.post(url, json=payload)
            resp.raise_for_status()
            logger.info("Triggering Recommender Service succeeded", url=url, status_code=resp.status_code)
    except Exception as e:
        logger.error("Failed to trigger Recommender Service webhook", url=url, error=str(e))

def process_rfq_matched(rfq_id: str):
    logger.info("Processing RFQ matched to generate quotes", rfq_id=rfq_id)
    try:
        with psycopg.connect(DATABASE_URL) as conn:
            with conn.cursor(row_factory=dict_row) as cur:
                # Fetch RFQ details (especially quantity)
                cur.execute("SELECT quantity FROM rfqs WHERE id = %s", (rfq_id,))
                rfq = cur.fetchone()
                if not rfq:
                    logger.error("RFQ not found in database", rfq_id=rfq_id)
                    return
                
                rfq_qty = rfq["quantity"]
                logger.info("Fetched RFQ details", rfq_id=rfq_id, quantity=rfq_qty)

                # Fetch matches and their supplier profiles
                cur.execute(
                    """
                    SELECT sp.id, sp.rating, sp.capacity_index, sp.base_lead_time, sp.risk_score, sp.auto_bid
                    FROM rfq_matches rm
                    JOIN supplier_profiles sp ON rm.supplier_id = sp.id
                    WHERE rm.rfq_id = %s
                    """,
                    (rfq_id,)
                )
                matches = cur.fetchall()
                logger.info("Found matching supplier profiles for quote generation", rfq_id=rfq_id, count=len(matches))

                # For each matched supplier, generate and insert a quote
                for m in matches:
                    sp_id = m["id"]
                    rating = float(m["rating"])
                    capacity_idx = float(m["capacity_index"])
                    base_lead_time = int(m["base_lead_time"])
                    supplier_risk = float(m["risk_score"])
                    auto_bid = bool(m["auto_bid"])

                    # Seed the numpy generator deterministically per RFQ and supplier combination
                    seed_bytes = hashlib.sha256(f"{rfq_id}-{sp_id}".encode()).digest()
                    seed = int.from_bytes(seed_bytes[:4], byteorder='big')
                    rng = np.random.default_rng(seed)

                    # 1. Unit Price calculation:
                    # premium for high rating, discount for high capacity index
                    rating_factor = 1.0 + (rating - 3.0) * 0.1
                    capacity_factor = 1.0 - (capacity_idx - 50.0) * 0.005
                    factor = max(0.5, rating_factor * capacity_factor)
                    
                    unit_price = rng.normal(loc=100.0 * factor, scale=15.0)
                    unit_price = float(np.round(max(10.0, unit_price), 2))

                    # 2. Total Price: unit price * quantity
                    total_price = float(np.round(unit_price * rfq_qty, 2))

                    # 3. Lead Time: Poisson distribution around base_lead_time
                    lead_time_days = int(rng.poisson(lam=base_lead_time))
                    lead_time_days = int(max(1, lead_time_days))

                    # 4. Risk Score: Normal distribution around supplier's risk score
                    risk_score = rng.normal(loc=supplier_risk, scale=0.05)
                    risk_score = float(np.round(np.clip(risk_score, 0.0, 1.0), 2))

                    # 5. Status: 'submitted' if auto_bid=true else 'suggested'
                    status = "submitted" if auto_bid else "suggested"

                    logger.info(
                        "Generated quote for supplier",
                        rfq_id=rfq_id,
                        supplier_id=sp_id,
                        unit_price=unit_price,
                        total_price=total_price,
                        lead_time=lead_time_days,
                        risk=risk_score,
                        status=status
                    )

                    # Insert quote
                    cur.execute(
                        """
                        INSERT INTO quotes (rfq_id, supplier_id, unit_price, total_price, lead_time_days, risk_score, status)
                        VALUES (%s, %s, %s, %s, %s, %s, %s)
                        ON CONFLICT DO NOTHING
                        """,
                        (rfq_id, sp_id, unit_price, total_price, lead_time_days, risk_score, status)
                    )
                
                # Update RFQ status to 'quotes_generated'
                cur.execute("UPDATE rfqs SET status = 'quotes_generated' WHERE id = %s", (rfq_id,))
                logger.info("Updated RFQ status to quotes_generated in database", rfq_id=rfq_id)
    except Exception as e:
        logger.exception("Database transaction failed in Quote Service", rfq_id=rfq_id, error=str(e))
        return

    # Callbacks (outside DB transaction block)
    send_status_callback(rfq_id, "quotes_generated")
    trigger_recommender_service(rfq_id)

@app.post("/webhook/rfq-matched")
async def rfq_matched(payload: RFQPayload, background_tasks: BackgroundTasks):
    logger.info("Received rfq-matched webhook", rfq_id=payload.rfq_id)
    background_tasks.add_task(process_rfq_matched, payload.rfq_id)
    return {"status": "processing"}

@app.get("/health")
def health():
    return {"status": "ok"}
