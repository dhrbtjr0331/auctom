package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type Broker struct {
	mu         sync.RWMutex
	clients    map[chan []byte]bool
	connect    chan chan []byte
	disconnect chan chan []byte
	broadcast  chan []byte
}

func NewBroker() *Broker {
	return &Broker{
		clients:    make(map[chan []byte]bool),
		connect:    make(chan chan []byte),
		disconnect: make(chan chan []byte),
		broadcast:  make(chan []byte),
	}
}

func (b *Broker) Start() {
	slog.Info("SSE broker started")
	for {
		select {
		case s := <-b.connect:
			b.mu.Lock()
			b.clients[s] = true
			b.mu.Unlock()
			slog.Info("New SSE client registered")
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
			for s := range b.clients {
				select {
				case s <- msg:
				default:
					// Don't block if client is slow
				}
			}
			b.mu.RUnlock()
		}
	}
}

func (b *Broker) BroadcastEvent(eventType string, data interface{}) {
	payload := map[string]interface{}{
		"event": eventType,
		"data":  data,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal SSE payload", "error", err)
		return
	}
	// Safely send to broadcast chan without blocking if broker is not running yet
	select {
	case b.broadcast <- bytes:
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	messageChan := make(chan []byte, 10)
	b.connect <- messageChan

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
