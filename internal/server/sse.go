package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// SSEMessage represents an individual event sent down an SSE stream.
type SSEMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type client struct {
	projectID int64
	ch        chan SSEMessage
}

// SSEHub coordinates real-time streaming connections to frontend clients.
type SSEHub struct {
	clients map[*client]bool
	mu      sync.RWMutex
}

// NewSSEHub creates a new SSE broadcast hub.
func NewSSEHub() *SSEHub {
	hub := &SSEHub{
		clients: make(map[*client]bool),
	}
	go hub.heartbeat()
	return hub
}

func (h *SSEHub) heartbeat() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.Broadcast(0, "ping", map[string]int64{"timestamp": time.Now().Unix()})
	}
}

// Broadcast sends a typed event to all connected clients listening on a project.
func (h *SSEHub) Broadcast(projectID int64, eventType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	msg := SSEMessage{
		Event: eventType,
		Data:  data,
	}

	for c := range h.clients {
		if c.projectID == 0 || projectID == 0 || c.projectID == projectID {
			select {
			case c.ch <- msg:
			default:
				// Channel full, skip to avoid blocking other clients
			}
		}
	}
}

// ServeHTTP handles the GET /api/v1/stream endpoint.
func (h *SSEHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	projStr := r.URL.Query().Get("project_id")
	var projectID int64
	if projStr != "" {
		projectID, _ = strconv.ParseInt(projStr, 10, 64)
	}

	c := &client{
		projectID: projectID,
		ch:        make(chan SSEMessage, 20),
	}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.ch)
	}()

	// Send initial connect notice
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.ch:
			if !ok {
				return
			}
			dataBytes, err := json.Marshal(msg.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, string(dataBytes))
			flusher.Flush()
		}
	}
}
