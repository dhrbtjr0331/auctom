import os
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

app = FastAPI(title="Recommender Service")

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

def process_quotes_generated(rfq_id: str):
    logger.info("Processing quotes ranking", rfq_id=rfq_id)
    try:
        with psycopg.connect(DATABASE_URL) as conn:
            with conn.cursor(row_factory=dict_row) as cur:
                # 1. Fetch RFQ priorities
                cur.execute(
                    "SELECT priority_cost, priority_lead_time, priority_risk FROM rfqs WHERE id = %s",
                    (rfq_id,)
                )
                rfq = cur.fetchone()
                if not rfq:
                    logger.error("RFQ not found in database", rfq_id=rfq_id)
                    return
                
                w_cost = float(rfq["priority_cost"])
                w_lead = float(rfq["priority_lead_time"])
                w_risk = float(rfq["priority_risk"])
                logger.info(
                    "Fetched RFQ priorities",
                    rfq_id=rfq_id,
                    w_cost=w_cost,
                    w_lead=w_lead,
                    w_risk=w_risk
                )

                # 2. Fetch all quotes for this RFQ
                cur.execute(
                    "SELECT id, total_price, lead_time_days, risk_score FROM quotes WHERE rfq_id = %s",
                    (rfq_id,)
                )
                quotes = cur.fetchall()
                
                if not quotes:
                    logger.warning("No quotes found to rank for this RFQ", rfq_id=rfq_id)
                    # Update status to ranked and return
                    cur.execute("UPDATE rfqs SET status = 'ranked' WHERE id = %s", (rfq_id,))
                    conn.commit()
                    send_status_callback(rfq_id, "ranked")
                    return

                # Convert to numpy arrays for calculation
                prices = np.array([float(q["total_price"]) for q in quotes])
                leads = np.array([float(q["lead_time_days"]) for q in quotes])
                risks = np.array([float(q["risk_score"]) for q in quotes])

                # Min/max normalization (mapping lower raw values to higher normalized scores)
                min_price, max_price = np.min(prices), np.max(prices)
                if max_price == min_price:
                    norm_prices = np.ones_like(prices)
                else:
                    norm_prices = (max_price - prices) / (max_price - min_price)

                min_lead, max_lead = np.min(leads), np.max(leads)
                if max_lead == min_lead:
                    norm_leads = np.ones_like(leads)
                else:
                    norm_leads = (max_lead - leads) / (max_lead - min_lead)

                min_risk, max_risk = np.min(risks), np.max(risks)
                if max_risk == min_risk:
                    norm_risks = np.ones_like(risks)
                else:
                    norm_risks = (max_risk - risks) / (max_risk - min_risk)

                # Calculate weighted scores
                scores = w_cost * norm_prices + w_lead * norm_leads + w_risk * norm_risks
                # Ensure the scores are strictly within [0.0, 1.0] and rounded to 4 decimal places (for NUMERIC(5,4))
                scores = np.round(np.clip(scores, 0.0, 1.0), 4)

                # Rank quotes (descending by score)
                ranked_quotes = []
                for q, score in zip(quotes, scores):
                    ranked_quotes.append((q["id"], float(score)))
                
                ranked_quotes.sort(key=lambda x: x[1], reverse=True)

                # Clean up any existing recommendations for this RFQ to ensure idempotency
                cur.execute("DELETE FROM recommendations WHERE rfq_id = %s", (rfq_id,))

                # Insert recommendations
                for rank, (quote_id, score) in enumerate(ranked_quotes, start=1):
                    logger.info(
                        "Inserting recommendation",
                        rfq_id=rfq_id,
                        quote_id=quote_id,
                        score=score,
                        rank=rank
                    )
                    cur.execute(
                        """
                        INSERT INTO recommendations (rfq_id, quote_id, score, rank)
                        VALUES (%s, %s, %s, %s)
                        """,
                        (rfq_id, quote_id, score, rank)
                    )

                # Update RFQ status to 'ranked'
                cur.execute("UPDATE rfqs SET status = 'ranked' WHERE id = %s", (rfq_id,))
                logger.info("Updated RFQ status to ranked in database", rfq_id=rfq_id)
    except Exception as e:
        logger.exception("Database transaction failed in Recommender Service", rfq_id=rfq_id, error=str(e))
        return

    # Callback (outside DB transaction block)
    send_status_callback(rfq_id, "ranked")

@app.post("/webhook/quotes-generated")
async def quotes_generated(payload: RFQPayload, background_tasks: BackgroundTasks):
    logger.info("Received quotes-generated webhook", rfq_id=payload.rfq_id)
    background_tasks.add_task(process_quotes_generated, payload.rfq_id)
    return {"status": "processing"}

@app.get("/health")
def health():
    return {"status": "ok"}
