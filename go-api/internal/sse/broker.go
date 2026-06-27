package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"auctom/go-api/internal/auth"
)

type ClientInfo struct {
	UserID string
	Role   string
}

type clientRegister struct {
	ch   chan []byte
	info ClientInfo
}

type broadcastMessage struct {
	payload      []byte
	targetUsers  map[string]bool // If empty, broadcast to all
}

type Broker struct {
	mu         sync.RWMutex
	clients    map[chan []byte]ClientInfo
	connect    chan clientRegister
	disconnect chan chan []byte
	broadcast  chan broadcastMessage
}

func NewBroker() *Broker {
	return &Broker{
		clients:    make(map[chan []byte]ClientInfo),
		connect:    make(chan clientRegister),
		disconnect: make(chan chan []byte),
		broadcast:  make(chan broadcastMessage),
	}
}

func (b *Broker) Start() {
	slog.Info("SSE broker started")
	for {
		select {
		case reg := <-b.connect:
			b.mu.Lock()
			b.clients[reg.ch] = reg.info
			b.mu.Unlock()
			slog.Info("New SSE client registered", "user_id", reg.info.UserID, "role", reg.info.Role)
		case s := <-b.disconnect:
			b.mu.Lock()
			if _, ok := b.clients[s]; ok {
				delete(b.clients, s)
				close(s)
				slog.Info("SSE client deregistered")
			}
			b.mu.Unlock()
		case msg := <-b.broadcast:
			b.mu.RLock()
			for s, info := range b.clients {
				// Filter: if targetUsers is not empty, only send if user is in targetUsers
				if len(msg.targetUsers) > 0 && !msg.targetUsers[info.UserID] {
					continue
				}
				select {
				case s <- msg.payload:
				default:
					// Don't block if client is slow
				}
			}
			b.mu.RUnlock()
		}
	}
}

func (b *Broker) BroadcastEvent(eventType string, data interface{}, targetUserIDs ...string) {
	payload := map[string]interface{}{
		"event": eventType,
		"data":  data,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal SSE payload", "error", err)
		return
	}

	targets := make(map[string]bool)
	for _, id := range targetUserIDs {
		if id != "" {
			targets[id] = true
		}
	}

	select {
	case b.broadcast <- broadcastMessage{payload: bytes, targetUsers: targets}:
	default:
		slog.Warn("Broker broadcast channel full or not running, event dropped")
	}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Read token from query parameters
	tokenStr := r.URL.Query().Get("token")
	var clientInfo ClientInfo

	if tokenStr != "" {
		claims, err := auth.ValidateToken(tokenStr)
		if err == nil {
			clientInfo.UserID = claims.UserID
			clientInfo.Role = claims.Role
		} else {
			slog.Warn("SSE connection failed JWT validation", "error", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		slog.Warn("SSE connection missing token query param")
		http.Error(w, "Unauthorized: token required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan []byte, 10)
	b.connect <- clientRegister{ch: messageChan, info: clientInfo}

	defer func() {
		b.disconnect <- messageChan
	}()

	// Send connection established event
	fmt.Fprintf(w, "data: %s\n\n", `{"event":"connected","data":"connection established"}`)
	flusher.Flush()

	for {
		select {
		case msg, ok := <-messageChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
