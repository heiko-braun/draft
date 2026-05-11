package reviewd

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Hub manages SSE connections grouped by repo, broadcasting events to subscribers.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[chan Event]struct{} // repoID → set of client channels
	logger  *Logger
}

// Event is an SSE event to broadcast.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// NewHub creates a new SSE hub.
func NewHub(logger *Logger) *Hub {
	return &Hub{
		clients: make(map[string]map[chan Event]struct{}),
		logger:  logger,
	}
}

// Broadcast sends an event to all clients subscribed to the given repo.
func (h *Hub) Broadcast(repoID, eventType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[repoID]
	if !ok {
		return
	}

	event := Event{Type: eventType, Data: data}
	for ch := range clients {
		select {
		case ch <- event:
		default:
			// Drop if buffer full — client is slow.
		}
	}
}

// Subscribe registers a client for events on a repo. Returns the event
// channel and a cleanup function.
func (h *Hub) Subscribe(repoID string) (chan Event, func()) {
	ch := make(chan Event, 64)

	h.mu.Lock()
	if h.clients[repoID] == nil {
		h.clients[repoID] = make(map[chan Event]struct{})
	}
	h.clients[repoID][ch] = struct{}{}
	h.mu.Unlock()

	cleanup := func() {
		h.mu.Lock()
		delete(h.clients[repoID], ch)
		if len(h.clients[repoID]) == 0 {
			delete(h.clients, repoID)
		}
		h.mu.Unlock()
		close(ch)
	}

	return ch, cleanup
}

// HandleSSE is the HTTP handler for the SSE endpoint.
func (h *Hub) HandleSSE(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner := r.PathValue("owner")
		repo := r.PathValue("repo")
		if owner == "" || repo == "" {
			writeErrorJSON(w, http.StatusBadRequest, "missing owner or repo")
			return
		}

		rp, err := store.GetOrCreateRepo(owner, repo)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeErrorJSON(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()

		ch, cleanup := h.Subscribe(rp.ID)
		defer cleanup()

		ctx := r.Context()
		heartbeat := NewSafeTicker(30)
		defer heartbeat.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(event.Data)
				w.Write([]byte("event: " + event.Type + "\ndata: " + string(data) + "\n\n"))
				flusher.Flush()
			case <-heartbeat.C():
				w.Write([]byte(":heartbeat\n\n"))
				flusher.Flush()
			}
		}
	}
}

// SafeTicker wraps time.Ticker with a method-based channel access.
type SafeTicker struct {
	ticker *ticker
}

type ticker struct {
	ch   chan time.Time
	done chan struct{}
}

// NewSafeTicker creates a ticker that fires every n seconds.
func NewSafeTicker(seconds int) *SafeTicker {
	t := &ticker{
		ch:   make(chan time.Time),
		done: make(chan struct{}),
	}
	go func() {
		tick := newTimeTicker(seconds)
		defer tick.Stop()
		for {
			select {
			case <-t.done:
				return
			case v := <-tick.C:
				select {
				case t.ch <- v:
				case <-t.done:
					return
				}
			}
		}
	}()
	return &SafeTicker{ticker: t}
}

func (t *SafeTicker) C() <-chan time.Time { return t.ticker.ch }
func (t *SafeTicker) Stop()               { close(t.ticker.done) }

func newTimeTicker(seconds int) *time.Ticker {
	return time.NewTicker(time.Duration(seconds) * time.Second)
}
