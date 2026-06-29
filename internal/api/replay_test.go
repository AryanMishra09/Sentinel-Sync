package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aryan-mishra/sentinel-sync/internal/replica"
	"github.com/aryan-mishra/sentinel-sync/internal/simulation"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newTestHandler() (*Handler, *gin.Engine) {
	r := replica.New("test", nil, nil)
	h := NewHandler(r, simulation.NewChaos(), simulation.NewSimulator(r))
	engine := gin.New()
	h.RegisterRoutes(engine)
	return h, engine
}

// TestReplayEmptyLog returns an empty graph when the op log has no entries.
func TestReplayEmptyLog(t *testing.T) {
	_, engine := newTestHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/replay?upto=0", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var snap map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	nodes, _ := snap["nodes"].([]any)
	if len(nodes) != 0 {
		t.Errorf("want 0 nodes for empty log, got %d", len(nodes))
	}
}

// TestReplayBadParam rejects non-integer and negative upto values.
func TestReplayBadParam(t *testing.T) {
	_, engine := newTestHandler()

	for _, q := range []string{"/replay?upto=abc", "/replay?upto=-1", "/replay"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", q, nil)
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("query %q: want 400, got %d", q, w.Code)
		}
	}
}

// TestReplayUptoClamp clamps out-of-bounds upto to the last op.
func TestReplayUptoClamp(t *testing.T) {
	h, engine := newTestHandler()

	// Create 3 nodes → op log has 3 entries (indices 0, 1, 2).
	for _, id := range []string{"n1", "n2", "n3"} {
		if _, err := h.replica.CreateNode(id, id, 0, 0); err != nil {
			t.Fatal(err)
		}
	}

	// Requesting upto=999 should clamp to 2 and still return all 3 nodes.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/replay?upto=999", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var snap map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var nodes []any
	if err := json.Unmarshal(snap["nodes"], &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("want 3 nodes after clamp, got %d", len(nodes))
	}
}

// TestReplayAtIndex verifies that requesting upto=0 returns only the first op's effect.
func TestReplayAtIndex(t *testing.T) {
	h, engine := newTestHandler()

	// Create 3 nodes sequentially.
	for _, id := range []string{"alpha", "beta", "gamma"} {
		if _, err := h.replica.CreateNode(id, id, 0, 0); err != nil {
			t.Fatal(err)
		}
	}

	for idx, wantNodes := range []int{1, 2, 3} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/replay?upto="+itoa(idx), nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("upto=%d: want 200, got %d", idx, w.Code)
		}
		var snap map[string]json.RawMessage
		json.Unmarshal(w.Body.Bytes(), &snap)
		var nodes []any
		json.Unmarshal(snap["nodes"], &nodes)
		if len(nodes) != wantNodes {
			t.Errorf("upto=%d: want %d nodes, got %d", idx, wantNodes, len(nodes))
		}
	}
}

func itoa(n int) string {
	return string(rune('0' + n))
}
