package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"auctom/go-api/internal/auth"
	"auctom/go-api/internal/sse"
)

type Handlers struct {
	DB                    *sql.DB
	Broker                *sse.Broker
	MatchingServiceURL    string
	QuoteServiceURL       string
	RecommenderServiceURL string
}

// Auth JSON structures
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email         string   `json:"email"`
	Password      string   `json:"password"`
	Role          string   `json:"role"` // 'buyer' or 'supplier'
	CompanyName   string   `json:"company_name,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	CapacityIndex int      `json:"capacity_index,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	BaseLeadTime  int      `json:"base_lead_time,omitempty"`
	RiskScore     float64  `json:"risk_score,omitempty"`
	AutoBid       bool     `json:"auto_bid,omitempty"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// RFQ structures
type RFQRequest struct {
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	PartName           string  `json:"part_name"`
	Quantity           int     `json:"quantity"`
	RequiredCapability string  `json:"required_capability"`
	PriorityCost       float64 `json:"priority_cost"`
	PriorityLeadTime   float64 `json:"priority_lead_time"`
	PriorityRisk       float64 `json:"priority_risk"`
	SpecSize           string  `json:"spec_size"`
	SpecMaterial       string  `json:"spec_material"`
	SpecNotes          string  `json:"spec_notes"`
}

type RFQResponse struct {
	ID                 string  `json:"id"`
	BuyerID            string  `json:"buyer_id"`
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	PartName           string  `json:"part_name"`
	Quantity           int     `json:"quantity"`
	RequiredCapability string  `json:"required_capability"`
	PriorityCost       float64 `json:"priority_cost"`
	PriorityLeadTime   float64 `json:"priority_lead_time"`
	PriorityRisk       float64 `json:"priority_risk"`
	Status             string  `json:"status"`
	AwardedQuoteID     *string `json:"awarded_quote_id,omitempty"`
	SpecSize           string  `json:"spec_size"`
	SpecMaterial       string  `json:"spec_material"`
	SpecNotes          string  `json:"spec_notes"`
	CreatedAt          string  `json:"created_at"`
}

type SupplierProfileResponse struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	CompanyName   string   `json:"company_name"`
	Capabilities  []string `json:"capabilities"`
	CapacityIndex int      `json:"capacity_index"`
	Rating        float64  `json:"rating"`
	BaseLeadTime  int      `json:"base_lead_time"`
	RiskScore     float64  `json:"risk_score"`
	AutoBid       bool     `json:"auto_bid"`
}

type QuoteResponse struct {
	ID           string  `json:"id"`
	RFQID        string  `json:"rfq_id"`
	SupplierID   string  `json:"supplier_id"`
	CompanyName  string  `json:"company_name"`
	UnitPrice    float64 `json:"unit_price"`
	TotalPrice   float64 `json:"total_price"`
	LeadTimeDays int     `json:"lead_time_days"`
	RiskScore    float64 `json:"risk_score"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

type RecommendationResponse struct {
	ID          string  `json:"id"`
	RFQID       string  `json:"rfq_id"`
	QuoteID     string  `json:"quote_id"`
	Score       float64 `json:"score"`
	Rank        int     `json:"rank"`
	CompanyName string  `json:"company_name"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
}

type RFQDetailResponse struct {
	RFQ             RFQResponse               `json:"rfq"`
	Matches         []SupplierProfileResponse `json:"matches"`
	Quotes          []QuoteResponse           `json:"quotes"`
	Recommendations []RecommendationResponse  `json:"recommendations"`
}

// Webhook payload structures
type WebhookPayload struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type MatchEventData struct {
	RFQID      string   `json:"rfq_id"`
	SupplierIDs []string `json:"supplier_ids"`
}

type QuoteEventData struct {
	RFQID  string       `json:"rfq_id"`
	Quotes []QuoteInput `json:"quotes"`
}

type QuoteInput struct {
	SupplierID   string  `json:"supplier_id"`
	UnitPrice    float64 `json:"unit_price"`
	TotalPrice   float64 `json:"total_price"`
	LeadTimeDays int     `json:"lead_time_days"`
	RiskScore    float64 `json:"risk_score"`
}

type RankEventData struct {
	RFQID           string            `json:"rfq_id"`
	Recommendations []RecommendationInput `json:"recommendations"`
}

type RecommendationInput struct {
	QuoteID string  `json:"quote_id"`
	Score   float64 `json:"score"`
	Rank    int     `json:"rank"`
}

// Helper to trigger external microservice calls asynchronously
func (h *Handlers) triggerService(serviceURL, endpoint string, payload interface{}) {
	go func() {
		data, err := json.Marshal(payload)
		if err != nil {
			slog.Error("Failed to marshal service payload", "endpoint", endpoint, "error", err)
			return
		}
		url := fmt.Sprintf("%s%s", serviceURL, endpoint)
		slog.Info("Triggering external service", "url", url)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			slog.Error("Failed to call service", "url", url, "error", err)
			return
		}
		defer resp.Body.Close()
		slog.Info("Service called successfully", "url", url, "status", resp.Status)
	}()
}

// POST /api/auth/register
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || (req.Role != "buyer" && req.Role != "supplier") {
		http.Error(w, `{"error": "Email, password, and valid role (buyer/supplier) are required"}`, http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error": "Password hashing failed"}`, http.StatusInternalServerError)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRow(`
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id`, req.Email, string(hashedPassword), req.Role).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, `{"error": "Email already in use"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"error": "Database error: `+err.Error()+`"}`, http.StatusInternalServerError)
		}
		return
	}

	if req.Role == "supplier" {
		if req.CompanyName == "" {
			req.CompanyName = "Unnamed Company"
		}
		if req.CapacityIndex < 1 {
			req.CapacityIndex = 50
		}
		if req.Rating < 1.0 {
			req.Rating = 4.0
		}
		if req.BaseLeadTime < 1 {
			req.BaseLeadTime = 7
		}
		if req.RiskScore < 0.0 {
			req.RiskScore = 0.2
		}

		_, err = tx.Exec(`
			INSERT INTO supplier_profiles (user_id, company_name, capabilities, capacity_index, rating, base_lead_time, risk_score, auto_bid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			userID, req.CompanyName, pq.Array(req.Capabilities), req.CapacityIndex, req.Rating, req.BaseLeadTime, req.RiskScore, req.AutoBid)
		if err != nil {
			http.Error(w, `{"error": "Failed to create supplier profile: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error": "Transaction commit failed"}`, http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(userID, req.Email, req.Role)
	if err != nil {
		http.Error(w, `{"error": "Token generation failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User: UserResponse{
			ID:    userID,
			Email: req.Email,
			Role:  req.Role,
		},
	})
}

// POST /api/auth/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	var userID, email, passwordHash, role string
	err := h.DB.QueryRow(`
		SELECT id, email, password_hash, role
		FROM users
		WHERE email = $1`, req.Email).Scan(&userID, &email, &passwordHash, &role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": "Invalid email or password"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error": "Invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(userID, email, role)
	if err != nil {
		http.Error(w, `{"error": "Token generation failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User: UserResponse{
			ID:    userID,
			Email: email,
			Role:  role,
		},
	})
}

// POST /api/rfqs
func (h *Handlers) CreateRFQ(w http.ResponseWriter, r *http.Request) {
	role := auth.GetUserRole(r.Context())
	if role != "buyer" {
		http.Error(w, `{"error": "Forbidden: Only buyers can create RFQs"}`, http.StatusForbidden)
		return
	}

	buyerID := auth.GetUserID(r.Context())

	var req RFQRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.PartName == "" || req.Quantity <= 0 || req.RequiredCapability == "" {
		http.Error(w, `{"error": "Title, part_name, quantity (>0), and required_capability are required"}`, http.StatusBadRequest)
		return
	}

	// Apply defaults for priority weights
	if req.PriorityCost == 0 && req.PriorityLeadTime == 0 && req.PriorityRisk == 0 {
		req.PriorityCost = 0.33
		req.PriorityLeadTime = 0.33
		req.PriorityRisk = 0.34
	}

	var rfq RFQResponse
	var desc, size, mat, notes sql.NullString
	var createdAtTime interface{}

	err := h.DB.QueryRow(`
		INSERT INTO rfqs (buyer_id, title, description, part_name, quantity, required_capability, priority_cost, priority_lead_time, priority_risk, status, spec_size, spec_material, spec_notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'created', $10, $11, $12)
		RETURNING id, buyer_id, title, description, part_name, quantity, required_capability, priority_cost, priority_lead_time, priority_risk, status, awarded_quote_id, spec_size, spec_material, spec_notes, created_at`,
		buyerID, req.Title, req.Description, req.PartName, req.Quantity, req.RequiredCapability, req.PriorityCost, req.PriorityLeadTime, req.PriorityRisk, req.SpecSize, req.SpecMaterial, req.SpecNotes).
		Scan(&rfq.ID, &rfq.BuyerID, &rfq.Title, &desc, &rfq.PartName, &rfq.Quantity, &rfq.RequiredCapability, &rfq.PriorityCost, &rfq.PriorityLeadTime, &rfq.PriorityRisk, &rfq.Status, &rfq.AwardedQuoteID, &size, &mat, &notes, &createdAtTime)

	if err != nil {
		slog.Error("Failed to insert RFQ", "error", err)
		http.Error(w, `{"error": "Database error: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	rfq.Description = desc.String
	rfq.SpecSize = size.String
	rfq.SpecMaterial = mat.String
	rfq.SpecNotes = notes.String
	if t, ok := createdAtTime.(fmt.Stringer); ok {
		rfq.CreatedAt = t.String()
	} else {
		rfq.CreatedAt = fmt.Sprintf("%v", createdAtTime)
	}

	slog.Info("RFQ created", "id", rfq.ID)
	h.Broker.BroadcastEvent("rfq.created", rfq)

	// Transition to matching status and trigger Matching service
	_, err = h.DB.Exec(`UPDATE rfqs SET status = 'matching' WHERE id = $1`, rfq.ID)
	if err != nil {
		slog.Error("Failed to update RFQ status to matching", "rfq_id", rfq.ID, "error", err)
	} else {
		rfq.Status = "matching"
		h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
			"rfq_id": rfq.ID,
			"status": "matching",
		})
	}

	h.triggerService(h.MatchingServiceURL, "/webhook/rfq-created", map[string]interface{}{
		"rfq_id": rfq.ID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rfq)
}

// GET /api/rfqs
func (h *Handlers) GetRFQs(w http.ResponseWriter, r *http.Request) {
	role := auth.GetUserRole(r.Context())
	userID := auth.GetUserID(r.Context())

	var rows *sql.Rows
	var err error

	if role == "buyer" {
		rows, err = h.DB.Query(`
			SELECT id, buyer_id, title, description, part_name, quantity, required_capability, priority_cost, priority_lead_time, priority_risk, status, awarded_quote_id, spec_size, spec_material, spec_notes, created_at
			FROM rfqs
			WHERE buyer_id = $1
			ORDER BY created_at DESC`, userID)
	} else {
		// Supplier or other: show all RFQs
		rows, err = h.DB.Query(`
			SELECT id, buyer_id, title, description, part_name, quantity, required_capability, priority_cost, priority_lead_time, priority_risk, status, awarded_quote_id, spec_size, spec_material, spec_notes, created_at
			FROM rfqs
			ORDER BY created_at DESC`)
	}

	if err != nil {
		slog.Error("Failed to fetch RFQs", "error", err)
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rfqs := []RFQResponse{}
	for rows.Next() {
		var rfq RFQResponse
		var desc, size, mat, notes sql.NullString
		var createdAtTime interface{}

		err = rows.Scan(&rfq.ID, &rfq.BuyerID, &rfq.Title, &desc, &rfq.PartName, &rfq.Quantity, &rfq.RequiredCapability, &rfq.PriorityCost, &rfq.PriorityLeadTime, &rfq.PriorityRisk, &rfq.Status, &rfq.AwardedQuoteID, &size, &mat, &notes, &createdAtTime)
		if err != nil {
			slog.Error("Failed to scan RFQ row", "error", err)
			continue
		}

		rfq.Description = desc.String
		rfq.SpecSize = size.String
		rfq.SpecMaterial = mat.String
		rfq.SpecNotes = notes.String
		if t, ok := createdAtTime.(fmt.Stringer); ok {
			rfq.CreatedAt = t.String()
		} else {
			rfq.CreatedAt = fmt.Sprintf("%v", createdAtTime)
		}

		rfqs = append(rfqs, rfq)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rfqs)
}

// GET /api/rfqs/{id}
func (h *Handlers) GetRFQByID(w http.ResponseWriter, r *http.Request) {
	rfqID := chi.URLParam(r, "id")

	var rfq RFQResponse
	var desc, size, mat, notes sql.NullString
	var createdAtTime interface{}

	err := h.DB.QueryRow(`
		SELECT id, buyer_id, title, description, part_name, quantity, required_capability, priority_cost, priority_lead_time, priority_risk, status, awarded_quote_id, spec_size, spec_material, spec_notes, created_at
		FROM rfqs
		WHERE id = $1`, rfqID).
		Scan(&rfq.ID, &rfq.BuyerID, &rfq.Title, &desc, &rfq.PartName, &rfq.Quantity, &rfq.RequiredCapability, &rfq.PriorityCost, &rfq.PriorityLeadTime, &rfq.PriorityRisk, &rfq.Status, &rfq.AwardedQuoteID, &size, &mat, &notes, &createdAtTime)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": "RFQ not found"}`, http.StatusNotFound)
		} else {
			slog.Error("Failed to fetch RFQ", "id", rfqID, "error", err)
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
	}

	rfq.Description = desc.String
	rfq.SpecSize = size.String
	rfq.SpecMaterial = mat.String
	rfq.SpecNotes = notes.String
	if t, ok := createdAtTime.(fmt.Stringer); ok {
		rfq.CreatedAt = t.String()
	} else {
		rfq.CreatedAt = fmt.Sprintf("%v", createdAtTime)
	}

	// Fetch matches
	matches := []SupplierProfileResponse{}
	mRows, err := h.DB.Query(`
		SELECT sp.id, sp.user_id, sp.company_name, sp.capabilities, sp.capacity_index, sp.rating, sp.base_lead_time, sp.risk_score, sp.auto_bid
		FROM rfq_matches rm
		JOIN supplier_profiles sp ON rm.supplier_id = sp.id
		WHERE rm.rfq_id = $1`, rfqID)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var sp SupplierProfileResponse
			var capabilities []string
			err = mRows.Scan(&sp.ID, &sp.UserID, &sp.CompanyName, pq.Array(&capabilities), &sp.CapacityIndex, &sp.Rating, &sp.BaseLeadTime, &sp.RiskScore, &sp.AutoBid)
			if err == nil {
				sp.Capabilities = capabilities
				matches = append(matches, sp)
			} else {
				slog.Error("Scan match row error", "error", err)
			}
		}
	}

	// Fetch quotes
	quotes := []QuoteResponse{}
	qRows, err := h.DB.Query(`
		SELECT q.id, q.rfq_id, q.supplier_id, sp.company_name, q.unit_price, q.total_price, q.lead_time_days, q.risk_score, q.status, q.created_at
		FROM quotes q
		JOIN supplier_profiles sp ON q.supplier_id = sp.id
		WHERE q.rfq_id = $1
		ORDER BY q.total_price ASC`, rfqID)
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var q QuoteResponse
			var qCreatedAtTime interface{}
			err = qRows.Scan(&q.ID, &q.RFQID, &q.SupplierID, &q.CompanyName, &q.UnitPrice, &q.TotalPrice, &q.LeadTimeDays, &q.RiskScore, &q.Status, &qCreatedAtTime)
			if err == nil {
				if t, ok := qCreatedAtTime.(fmt.Stringer); ok {
					q.CreatedAt = t.String()
				} else {
					q.CreatedAt = fmt.Sprintf("%v", qCreatedAtTime)
				}
				quotes = append(quotes, q)
			} else {
				slog.Error("Scan quote row error", "error", err)
			}
		}
	}

	// Fetch recommendations
	recs := []RecommendationResponse{}
	rRows, err := h.DB.Query(`
		SELECT r.id, r.rfq_id, r.quote_id, r.score, r.rank, sp.company_name, q.unit_price, q.total_price
		FROM recommendations r
		JOIN quotes q ON r.quote_id = q.id
		JOIN supplier_profiles sp ON q.supplier_id = sp.id
		WHERE r.rfq_id = $1
		ORDER BY r.rank ASC`, rfqID)
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var rec RecommendationResponse
			err = rRows.Scan(&rec.ID, &rec.RFQID, &rec.QuoteID, &rec.Score, &rec.Rank, &rec.CompanyName, &rec.UnitPrice, &rec.TotalPrice)
			if err == nil {
				recs = append(recs, rec)
			} else {
				slog.Error("Scan recommendation row error", "error", err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RFQDetailResponse{
		RFQ:             rfq,
		Matches:         matches,
		Quotes:          quotes,
		Recommendations: recs,
	})
}

// POST /api/rfqs/{id}/award
func (h *Handlers) AwardRFQ(w http.ResponseWriter, r *http.Request) {
	role := auth.GetUserRole(r.Context())
	if role != "buyer" {
		http.Error(w, `{"error": "Forbidden: Only buyers can award RFQs"}`, http.StatusForbidden)
		return
	}

	rfqID := chi.URLParam(r, "id")
	buyerID := auth.GetUserID(r.Context())

	var req struct {
		QuoteID string `json:"quote_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.QuoteID == "" {
		http.Error(w, `{"error": "quote_id is required"}`, http.StatusBadRequest)
		return
	}

	// Ensure the RFQ exists and belongs to the buyer
	var currentBuyerID string
	err := h.DB.QueryRow(`SELECT buyer_id FROM rfqs WHERE id = $1`, rfqID).Scan(&currentBuyerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": "RFQ not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
	}

	if currentBuyerID != buyerID {
		http.Error(w, `{"error": "Unauthorized: This RFQ belongs to another buyer"}`, http.StatusUnauthorized)
		return
	}

	// Begin transaction to award the quote
	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update RFQ status and awarded_quote_id
	_, err = tx.Exec(`
		UPDATE rfqs
		SET status = 'awarded', awarded_quote_id = $1
		WHERE id = $2`, req.QuoteID, rfqID)
	if err != nil {
		slog.Error("Failed to award RFQ", "rfq_id", rfqID, "quote_id", req.QuoteID, "error", err)
		http.Error(w, `{"error": "Failed to update RFQ status"}`, http.StatusInternalServerError)
		return
	}

	// Update selected quote's status to 'awarded'
	_, err = tx.Exec(`
		UPDATE quotes
		SET status = 'awarded'
		WHERE id = $1 AND rfq_id = $2`, req.QuoteID, rfqID)
	if err != nil {
		slog.Error("Failed to update quote status to awarded", "quote_id", req.QuoteID, "error", err)
		http.Error(w, `{"error": "Failed to update quote status"}`, http.StatusInternalServerError)
		return
	}

	// Update other quotes for this RFQ to 'rejected'
	_, err = tx.Exec(`
		UPDATE quotes
		SET status = 'rejected'
		WHERE rfq_id = $1 AND id != $2`, rfqID, req.QuoteID)
	if err != nil {
		slog.Error("Failed to reject other quotes", "rfq_id", rfqID, "error", err)
		// We log the error but don't fail the whole transaction as it's secondary
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error": "Transaction commit failed"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("RFQ awarded", "rfq_id", rfqID, "quote_id", req.QuoteID)

	// Broadcast SSE updates
	h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
		"rfq_id":           rfqID,
		"status":           "awarded",
		"awarded_quote_id": req.QuoteID,
	})
	h.Broker.BroadcastEvent("quote.status_updated", map[string]string{
		"rfq_id":   rfqID,
		"quote_id": req.QuoteID,
		"status":   "awarded",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "RFQ successfully awarded"})
}

// POST /api/quotes/{id}/submit (Manual submit by supplier)
func (h *Handlers) SubmitQuote(w http.ResponseWriter, r *http.Request) {
	role := auth.GetUserRole(r.Context())
	if role != "supplier" {
		http.Error(w, `{"error": "Forbidden: Only suppliers can submit quotes"}`, http.StatusForbidden)
		return
	}

	userID := auth.GetUserID(r.Context())
	quoteID := chi.URLParam(r, "id")

	// Verify supplier profile and that this quote belongs to them
	var quoteSupplierID, profileUserID, rfqID string
	err := h.DB.QueryRow(`
		SELECT q.supplier_id, q.rfq_id, sp.user_id
		FROM quotes q
		JOIN supplier_profiles sp ON q.supplier_id = sp.id
		WHERE q.id = $1`, quoteID).Scan(&quoteSupplierID, &rfqID, &profileUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"error": "Quote not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		}
		return
	}

	if profileUserID != userID {
		http.Error(w, `{"error": "Unauthorized: This quote does not belong to your company"}`, http.StatusUnauthorized)
		return
	}

	_, err = h.DB.Exec(`UPDATE quotes SET status = 'submitted' WHERE id = $1`, quoteID)
	if err != nil {
		slog.Error("Failed to submit quote", "quote_id", quoteID, "error", err)
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("Quote submitted manually", "quote_id", quoteID)
	h.Broker.BroadcastEvent("quote.status_updated", map[string]string{
		"rfq_id":   rfqID,
		"quote_id": quoteID,
		"status":   "submitted",
	})

	// Trigger recommender service to recalculate recommendations based on new submitted quote
	h.triggerService(h.RecommenderServiceURL, "/recommend", map[string]interface{}{
		"rfq_id": rfqID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Quote successfully submitted"})
}

// POST /api/webhooks (Webhook dispatcher)
func (h *Handlers) WebhookDispatcher(w http.ResponseWriter, r *http.Request) {
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid webhook payload"}`, http.StatusBadRequest)
		return
	}

	slog.Info("Webhook received", "event", payload.Event)

	switch payload.Event {
	case "rfq.matched":
		var matchData MatchEventData
		if err := json.Unmarshal(payload.Data, &matchData); err != nil {
			http.Error(w, `{"error": "Invalid event data for rfq.matched"}`, http.StatusBadRequest)
			return
		}

		if matchData.RFQID == "" {
			http.Error(w, `{"error": "rfq_id is required"}`, http.StatusBadRequest)
			return
		}

		tx, err := h.DB.Begin()
		if err != nil {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Save matches in rfq_matches
		for _, supplierID := range matchData.SupplierIDs {
			_, err = tx.Exec(`
				INSERT INTO rfq_matches (rfq_id, supplier_id)
				VALUES ($1, $2)
				ON CONFLICT (rfq_id, supplier_id) DO NOTHING`, matchData.RFQID, supplierID)
			if err != nil {
				slog.Error("Failed to insert match", "rfq_id", matchData.RFQID, "supplier_id", supplierID, "error", err)
			}
		}

		// Update RFQ status to 'matched' (or quotes_generated/etc. depending on matches)
		_, err = tx.Exec(`UPDATE rfqs SET status = 'matched' WHERE id = $1`, matchData.RFQID)
		if err != nil {
			slog.Error("Failed to update RFQ status to matched", "rfq_id", matchData.RFQID, "error", err)
			http.Error(w, `{"error": "Failed to update RFQ status"}`, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, `{"error": "Transaction commit failed"}`, http.StatusInternalServerError)
			return
		}

		h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
			"rfq_id": matchData.RFQID,
			"status": "matched",
		})

		// Trigger Quote generation service
		h.triggerService(h.QuoteServiceURL, "/generate-quotes", map[string]interface{}{
			"rfq_id": matchData.RFQID,
		})

	case "quotes.generated":
		var quoteData QuoteEventData
		if err := json.Unmarshal(payload.Data, &quoteData); err != nil {
			http.Error(w, `{"error": "Invalid event data for quotes.generated"}`, http.StatusBadRequest)
			return
		}

		if quoteData.RFQID == "" {
			http.Error(w, `{"error": "rfq_id is required"}`, http.StatusBadRequest)
			return
		}

		tx, err := h.DB.Begin()
		if err != nil {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Insert quotes, applying auto-bid checks
		for _, q := range quoteData.Quotes {
			var autoBid bool
			err = tx.QueryRow(`
				SELECT auto_bid
				FROM supplier_profiles
				WHERE id = $1`, q.SupplierID).Scan(&autoBid)
			if err != nil {
				slog.Warn("Failed to find supplier profile for auto-bid check", "supplier_id", q.SupplierID, "error", err)
				autoBid = false
			}

			status := "suggested"
			if autoBid {
				status = "submitted"
			}

			_, err = tx.Exec(`
				INSERT INTO quotes (rfq_id, supplier_id, unit_price, total_price, lead_time_days, risk_score, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT DO NOTHING`,
				quoteData.RFQID, q.SupplierID, q.UnitPrice, q.TotalPrice, q.LeadTimeDays, q.RiskScore, status)
			if err != nil {
				slog.Error("Failed to insert quote", "rfq_id", quoteData.RFQID, "supplier_id", q.SupplierID, "error", err)
			}
		}

		_, err = tx.Exec(`UPDATE rfqs SET status = 'quotes_generated' WHERE id = $1`, quoteData.RFQID)
		if err != nil {
			slog.Error("Failed to update RFQ status to quotes_generated", "rfq_id", quoteData.RFQID, "error", err)
			http.Error(w, `{"error": "Failed to update RFQ status"}`, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, `{"error": "Transaction commit failed"}`, http.StatusInternalServerError)
			return
		}

		h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
			"rfq_id": quoteData.RFQID,
			"status": "quotes_generated",
		})
		h.Broker.BroadcastEvent("quotes.generated", map[string]string{
			"rfq_id": quoteData.RFQID,
		})

		// Trigger Recommender service
		h.triggerService(h.RecommenderServiceURL, "/recommend", map[string]interface{}{
			"rfq_id": quoteData.RFQID,
		})

	case "rfq.ranked":
		var rankData RankEventData
		if err := json.Unmarshal(payload.Data, &rankData); err != nil {
			http.Error(w, `{"error": "Invalid event data for rfq.ranked"}`, http.StatusBadRequest)
			return
		}

		if rankData.RFQID == "" {
			http.Error(w, `{"error": "rfq_id is required"}`, http.StatusBadRequest)
			return
		}

		tx, err := h.DB.Begin()
		if err != nil {
			http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		for _, rec := range rankData.Recommendations {
			_, err = tx.Exec(`
				INSERT INTO recommendations (rfq_id, quote_id, score, rank)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (quote_id)
				DO UPDATE SET score = EXCLUDED.score, rank = EXCLUDED.rank`,
				rankData.RFQID, rec.QuoteID, rec.Score, rec.Rank)
			if err != nil {
				slog.Error("Failed to insert recommendation", "rfq_id", rankData.RFQID, "quote_id", rec.QuoteID, "error", err)
			}
		}

		_, err = tx.Exec(`UPDATE rfqs SET status = 'ranked' WHERE id = $1`, rankData.RFQID)
		if err != nil {
			slog.Error("Failed to update RFQ status to ranked", "rfq_id", rankData.RFQID, "error", err)
			http.Error(w, `{"error": "Failed to update RFQ status"}`, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, `{"error": "Transaction commit failed"}`, http.StatusInternalServerError)
			return
		}

		h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
			"rfq_id": rankData.RFQID,
			"status": "ranked",
		})
		h.Broker.BroadcastEvent("rfq.ranked", map[string]string{
			"rfq_id": rankData.RFQID,
		})

	default:
		slog.Warn("Unhandled webhook event", "event", payload.Event)
		http.Error(w, `{"error": "Unhandled event type"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "success"}`))
}

type StatusCallbackRequest struct {
	RFQID  string `json:"rfq_id"`
	Status string `json:"status"`
}

// POST /api/webhooks/status-callback
func (h *Handlers) StatusCallback(w http.ResponseWriter, r *http.Request) {
	var req StatusCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.RFQID == "" || req.Status == "" {
		http.Error(w, `{"error": "rfq_id and status are required"}`, http.StatusBadRequest)
		return
	}

	// Update RFQ status
	_, err := h.DB.Exec(`UPDATE rfqs SET status = $1 WHERE id = $2`, req.Status, req.RFQID)
	if err != nil {
		slog.Error("Failed to update RFQ status in callback", "rfq_id", req.RFQID, "status", req.Status, "error", err)
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("RFQ status updated via microservice callback", "rfq_id", req.RFQID, "status", req.Status)

	// Broadcast event to SSE clients
	h.Broker.BroadcastEvent("rfq.status_updated", map[string]string{
		"rfq_id": req.RFQID,
		"status": req.Status,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
