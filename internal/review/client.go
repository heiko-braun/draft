package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Compile-time interface checks.
var (
	_ ReviewStore  = (*Client)(nil)
	_ ReviewSyncer = (*Client)(nil)
)

// Client is an HTTP client that implements ReviewStore and ReviewSyncer
// by delegating to the reviewd API.
type Client struct {
	baseURL  string // e.g. "http://localhost:5100"
	owner    string // GitHub owner
	repo     string // GitHub repo name
	token    string // GitHub OAuth token
	http     *http.Client
	versions map[string]int // threadID → last known version
}

// NewClient creates a new reviewd API client.
func NewClient(baseURL, owner, repo, token string) *Client {
	return &Client{
		baseURL:  baseURL,
		owner:    owner,
		repo:     repo,
		token:    token,
		http:     &http.Client{Timeout: 30 * time.Second},
		versions: make(map[string]int),
	}
}

func (c *Client) apiURL(path string) string {
	return fmt.Sprintf("%s/api/v1/repos/%s/%s%s", c.baseURL, c.owner, c.repo, path)
}

func (c *Client) doJSON(method, url string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// --- ReviewStore implementation ---

// threadWithVersion extends Thread with the server's version counter.
type threadWithVersion struct {
	Thread
	Version int `json:"version"`
}

func (c *Client) ListThreadsByDocument(document string) ([]Thread, error) {
	u := c.apiURL("/threads") + "?document=" + url.QueryEscape(document)
	var raw []threadWithVersion
	if err := c.doJSON("GET", u, nil, &raw); err != nil {
		return nil, err
	}
	threads := make([]Thread, len(raw))
	for i, r := range raw {
		c.versions[r.ID] = r.Version
		threads[i] = r.Thread
	}
	return threads, nil
}

func (c *Client) ListAllThreads() ([]Thread, error) {
	var raw []threadWithVersion
	if err := c.doJSON("GET", c.apiURL("/threads"), nil, &raw); err != nil {
		return nil, err
	}
	threads := make([]Thread, len(raw))
	for i, r := range raw {
		c.versions[r.ID] = r.Version
		threads[i] = r.Thread
	}
	return threads, nil
}

func (c *Client) CreateThread(reviewID, document string, anchor Anchor) (*Thread, error) {
	body := map[string]interface{}{
		"document":  document,
		"anchor":    anchor,
		"review_id": reviewID,
	}
	var thread Thread
	if err := c.doJSON("PUT", c.apiURL("/threads/"+newUUIDMust()), body, &thread); err != nil {
		return nil, err
	}
	return &thread, nil
}

func (c *Client) GetThread(document, threadID string) (*Thread, error) {
	var raw threadWithVersion
	if err := c.doJSON("GET", c.apiURL("/threads/"+threadID), nil, &raw); err != nil {
		return nil, err
	}
	c.versions[raw.ID] = raw.Version
	return &raw.Thread, nil
}

func (c *Client) AddComment(document, threadID, author, body string) (*Comment, error) {
	reqBody := map[string]string{"body": body}
	var comment Comment
	if err := c.doJSON("POST", c.apiURL("/threads/"+threadID+"/comments"), reqBody, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

func (c *Client) ResolveThread(document, threadID string) error {
	return c.updateThreadStatus(threadID, "resolved")
}

func (c *Client) ReopenThread(document, threadID string) error {
	return c.updateThreadStatus(threadID, "open")
}

func (c *Client) updateThreadStatus(threadID, status string) error {
	version, ok := c.versions[threadID]
	if !ok {
		// Fetch to get current version.
		if _, err := c.GetThread("", threadID); err != nil {
			return err
		}
		version = c.versions[threadID]
	}

	body := map[string]string{"status": status}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", c.apiURL("/threads/"+threadID), bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", fmt.Sprintf("%d", version))
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update status failed: %s", string(respBody))
	}

	// Optimistically increment cached version.
	c.versions[threadID] = version + 1
	return nil
}

func (c *Client) DeleteThread(document, threadID string) error {
	req, _ := http.NewRequest("DELETE", c.apiURL("/threads/"+threadID), nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", string(respBody))
	}
	return nil
}

func (c *Client) ListReviews() ([]Review, error) {
	var reviews []Review
	if err := c.doJSON("GET", c.apiURL("/reviews"), nil, &reviews); err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []Review{}
	}
	return reviews, nil
}

func (c *Client) ListOpenReviews() ([]Review, error) {
	reviews, err := c.ListReviews()
	if err != nil {
		return nil, err
	}
	var open []Review
	for _, r := range reviews {
		if r.Status == ReviewOpen {
			open = append(open, r)
		}
	}
	return open, nil
}

func (c *Client) CreateReview(title string, documents []string, sourceRef string) (*Review, error) {
	body := map[string]interface{}{
		"title":      title,
		"documents":  documents,
		"source_ref": sourceRef,
	}
	var rev Review
	if err := c.doJSON("POST", c.apiURL("/reviews"), body, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// --- ReviewSyncer implementation ---

// SyncAll is a no-op — data lives on the server.
func (c *Client) SyncAll() error { return nil }

// Publish is a no-op — mutations are sent immediately.
func (c *Client) Publish() error { return nil }

// HasPendingChanges always returns false — no local queue.
func (c *Client) HasPendingChanges() (bool, error) { return false, nil }

// --- Helpers ---

func newUUIDMust() string {
	id, err := newUUID()
	if err != nil {
		panic(err)
	}
	return id
}
