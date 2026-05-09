package reviewd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heiko-braun/draft/internal/review"
)

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewHub(NewLogger("error"))

	ch1, cleanup1 := hub.Subscribe("repo-1")
	defer cleanup1()
	ch2, cleanup2 := hub.Subscribe("repo-1")
	defer cleanup2()
	ch3, cleanup3 := hub.Subscribe("repo-2")
	defer cleanup3()

	hub.Broadcast("repo-1", "thread.created", map[string]string{"id": "t1"})

	// Both repo-1 clients should receive.
	select {
	case e := <-ch1:
		if e.Type != "thread.created" {
			t.Errorf("ch1 type = %q", e.Type)
		}
	case <-time.After(time.Second):
		t.Error("ch1 timeout")
	}

	select {
	case e := <-ch2:
		if e.Type != "thread.created" {
			t.Errorf("ch2 type = %q", e.Type)
		}
	case <-time.After(time.Second):
		t.Error("ch2 timeout")
	}

	// repo-2 client should not receive.
	select {
	case <-ch3:
		t.Error("ch3 should not receive repo-1 events")
	case <-time.After(100 * time.Millisecond):
		// Expected.
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub(NewLogger("error"))

	ch, cleanup := hub.Subscribe("repo-1")
	cleanup()

	// After cleanup, broadcast should not block or panic.
	hub.Broadcast("repo-1", "test", nil)

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed")
		}
	default:
		// Channel is closed, which is correct.
	}
}

func TestHub_SlowClientDropped(t *testing.T) {
	hub := NewHub(NewLogger("error"))

	ch, cleanup := hub.Subscribe("repo-1")
	defer cleanup()

	// Fill the buffer (capacity 64).
	for i := 0; i < 100; i++ {
		hub.Broadcast("repo-1", "test", i)
	}

	// Should have 64 events, rest dropped.
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Errorf("got %d events, want 64 (buffer size)", count)
	}
}

func TestSSE_EventStream(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	logger := NewLogger("error")
	srv := NewServer(db, Config{}, logger)
	srv.store.GetOrCreateParticipant("abc123", "Test User", "test@example.com")

	// Start an SSE connection in a goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/api/v1/repos/sseorg/sserepo/events", nil)
	req = req.WithContext(ctx)
	req = req.WithContext(ContextWithAuth(req.Context(), &AuthContext{
		GitHubLogin:   "testuser",
		ParticipantID: "abc123",
		Email:         "test@example.com",
		Name:          "Test",
	}))

	// Use a pipe to read the SSE stream.
	w := &flushRecorder{header: make(http.Header), body: &bytes.Buffer{}, flushed: make(chan struct{}, 100)}

	go func() {
		srv.mux.ServeHTTP(w, req)
	}()

	// Wait for initial flush (headers sent).
	select {
	case <-w.flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE connection")
	}

	// Create a thread via the API — should emit an event.
	repo, _ := srv.store.GetOrCreateRepo("sseorg", "sserepo")
	thread := &review.Thread{Document: "docs/sse.md", Anchor: review.Anchor{Excerpt: "test"}}
	created, _ := srv.store.CreateThread(repo.ID, thread)

	srv.hub.Broadcast(repo.ID, "thread.created", created)

	// Wait for the event to arrive.
	time.Sleep(200 * time.Millisecond)
	cancel() // Close the SSE connection.

	// Parse the SSE stream.
	scanner := bufio.NewScanner(w.body)
	var foundEvent bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: thread.created") {
			foundEvent = true
		}
	}

	if !foundEvent {
		t.Errorf("did not find thread.created event in SSE stream, body: %s", w.body.String())
	}
}

// flushRecorder is a ResponseWriter that supports Flusher for SSE testing.
type flushRecorder struct {
	header  http.Header
	body    *bytes.Buffer
	status  int
	flushed chan struct{}
}

func (f *flushRecorder) Header() http.Header         { return f.header }
func (f *flushRecorder) WriteHeader(code int)        { f.status = code }
func (f *flushRecorder) Write(b []byte) (int, error) { return f.body.Write(b) }
func (f *flushRecorder) Flush() {
	select {
	case f.flushed <- struct{}{}:
	default:
	}
}

func TestSSE_EventPayload(t *testing.T) {
	hub := NewHub(NewLogger("error"))

	ch, cleanup := hub.Subscribe("repo-1")
	defer cleanup()

	data := map[string]string{"id": "thread-1", "status": "resolved"}
	hub.Broadcast("repo-1", "thread.resolved", data)

	select {
	case event := <-ch:
		if event.Type != "thread.resolved" {
			t.Errorf("type = %q, want thread.resolved", event.Type)
		}
		// Verify data roundtrips through JSON.
		jsonData, _ := json.Marshal(event.Data)
		var decoded map[string]string
		json.Unmarshal(jsonData, &decoded)
		if decoded["id"] != "thread-1" {
			t.Errorf("data.id = %q", decoded["id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
