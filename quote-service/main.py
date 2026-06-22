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

def calculate_direct_cost(part_name: str, spec_material: str, spec_size: str) -> float:
    # Base material/labor cost per unit
    base_unit_cost = 45.0
    
    # Material markup
    mat = (spec_material or "").lower()
    if "titanium" in mat or "inconel" in mat:
        base_unit_cost += 75.0
    elif "stainless" in mat or "tool steel" in mat:
        base_unit_cost += 35.0
    elif "aluminum" in mat:
        base_unit_cost += 15.0
    elif "plastic" in mat or "abs" in mat or "pla" in mat:
        base_unit_cost += 5.0
        
    # Size/volume markup
    sz = (spec_size or "").lower()
    if "large" in sz or "heavy" in sz:
        base_unit_cost += 40.0
    elif "medium" in sz:
        base_unit_cost += 15.0
        
    # Part name complexity markup
    pn = (part_name or "").lower()
    if "custom" in pn or "complex" in pn or "impeller" in pn or "turbine" in pn:
        base_unit_cost += 50.0
    elif "shaft" in pn or "bracket" in pn or "plate" in pn:
        base_unit_cost += 10.0
        
    return base_unit_cost

def process_rfq_matched(rfq_id: str):
    logger.info("Processing RFQ matched to generate quotes", rfq_id=rfq_id)
    try:
        with psycopg.connect(DATABASE_URL) as conn:
            with conn.cursor(row_factory=dict_row) as cur:
                # Fetch RFQ details
                cur.execute(
                    "SELECT part_name, quantity, spec_size, spec_material, spec_notes FROM rfqs WHERE id = %s",
                    (rfq_id,)
                )
                rfq = cur.fetchone()
                if not rfq:
                    logger.error("RFQ not found in database", rfq_id=rfq_id)
                    return
                
                part_name = rfq["part_name"]
                rfq_qty = rfq["quantity"]
                spec_size = rfq["spec_size"]
                spec_material = rfq["spec_material"]
                spec_notes = rfq["spec_notes"]
                logger.info("Fetched RFQ details", rfq_id=rfq_id, part_name=part_name, quantity=rfq_qty)

                # Fetch matches and their supplier profiles (including strategy fields)
                cur.execute(
                    """
                    SELECT sp.id, sp.rating, sp.capacity_index, sp.base_lead_time, sp.risk_score, sp.auto_bid,
                           sp.target_margin, sp.utilization_rate, sp.risk_tolerance, sp.agent_tier, sp.company_name
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
                    target_margin = float(m["target_margin"])
                    utilization_rate = float(m["utilization_rate"])
                    risk_tolerance = float(m["risk_tolerance"])
                    agent_tier = str(m["agent_tier"])
                    company_name = str(m["company_name"])

                    # Seed the numpy generator deterministically per RFQ and supplier combination
                    seed_bytes = hashlib.sha256(f"{rfq_id}-{sp_id}".encode()).digest()
                    seed = int.from_bytes(seed_bytes[:4], byteorder='big')
                    rng = np.random.default_rng(seed)

                    unit_price = 0.0
                    total_price = 0.0
                    lead_time_days = 0
                    risk_score = 0.0
                    gemini_success = False

                    if agent_tier == "premium":
                        api_key = os.environ.get("GEMINI_API_KEY")
                        if api_key:
                            logger.info("Calling Gemini API for premium bidding strategy", supplier_id=sp_id, company_name=company_name)
                            prompt = (
                                f"You are the AI Sales Agent for {company_name}. "
                                f"The buyer has requested a quote for {part_name} (Quantity: {rfq_qty}) "
                                f"with spec notes: '{spec_notes or 'None'}', material: '{spec_material or 'None'}', size: '{spec_size or 'None'}'. "
                                f"Your company's machine utilization is {int(utilization_rate * 100)}% and target margin is {int(target_margin * 100)}%. "
                                f"Analyze the RFQ complexity and recommend a competitive total price, lead time, and risk assessment. "
                                f"You must respond ONLY with a JSON object containing keys: 'unit_price' (float), 'total_price' (float), "
                                f"'lead_time_days' (int), and 'risk_score' (float, between 0.0 and 1.0)."
                            )
                            try:
                                url = f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key={api_key}"
                                headers = {"Content-Type": "application/json"}
                                payload = {
                                    "contents": [{
                                        "parts": [{"text": prompt}]
                                    }],
                                    "generationConfig": {
                                        "responseMimeType": "application/json",
                                        "responseSchema": {
                                            "type": "OBJECT",
                                            "properties": {
                                                "unit_price": {"type": "NUMBER"},
                                                "total_price": {"type": "NUMBER"},
                                                "lead_time_days": {"type": "INTEGER"},
                                                "risk_score": {"type": "NUMBER"}
                                            },
                                            "required": ["unit_price", "total_price", "lead_time_days", "risk_score"]
                                        }
                                    }
                                }
                                with httpx.Client(timeout=15.0) as client:
                                    resp = client.post(url, json=payload, headers=headers)
                                    resp.raise_for_status()
                                    resp_data = resp.json()
                                    
                                    candidates = resp_data.get("candidates", [])
                                    if candidates:
                                        text_content = candidates[0].get("content", {}).get("parts", [{}])[0].get("text", "")
                                        import json as python_json
                                        gemini_res = python_json.loads(text_content.strip())
                                        
                                        unit_price = float(gemini_res["unit_price"])
                                        total_price = float(gemini_res["total_price"])
                                        lead_time_days = int(gemini_res["lead_time_days"])
                                        risk_score = float(gemini_res["risk_score"])
                                        gemini_success = True
                                        logger.info("Gemini premium bid successfully generated", supplier_id=sp_id, unit_price=unit_price, total_price=total_price)
                            except Exception as gem_err:
                                logger.error("Gemini API invocation or parsing failed, falling back to Freemium rules", supplier_id=sp_id, error=str(gem_err))
                        else:
                            logger.warn("GEMINI_API_KEY is not set, falling back to Freemium rules", supplier_id=sp_id)

                    # Rules-based math bidding (Freemium or fallback)
                    if not gemini_success:
                        logger.info("Running rules-based bidding calculation", supplier_id=sp_id)
                        direct_cost = calculate_direct_cost(part_name, spec_material, spec_size)
                        
                        # Apply capacity factors to direct cost
                        rating_factor = 1.0 + (rating - 3.0) * 0.1
                        capacity_factor = 1.0 - (capacity_idx - 50.0) * 0.005
                        factor = max(0.5, rating_factor * capacity_factor)
                        direct_cost = direct_cost * factor
                        
                        base_unit_price = (direct_cost * (1.0 + target_margin)) * (1.0 + (utilization_rate ** 2) * 0.3)
                        
                        # Risk premium: High tolerance reduces the contingency premium
                        risk_premium = base_unit_price * (supplier_risk * (1.0 - risk_tolerance) * 0.2)
                        
                        unit_price = float(np.round(base_unit_price + risk_premium, 2))
                        unit_price = float(max(10.0, unit_price))
                        total_price = float(np.round(unit_price * rfq_qty, 2))
                        
                        lead_time_days = int(np.round(base_lead_time * (1.0 + utilization_rate * 1.5)))
                        lead_time_days = int(max(1, lead_time_days))
                        
                        risk_score = rng.normal(loc=supplier_risk, scale=0.05)
                        risk_score = float(np.round(np.clip(risk_score, 0.0, 1.0), 2))

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
