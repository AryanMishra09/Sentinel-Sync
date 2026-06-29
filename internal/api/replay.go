package api

import (
	"net/http"
	"strconv"

	"github.com/aryan-mishra/sentinel-sync/internal/replica"
	"github.com/gin-gonic/gin"
)

// handleReplay materializes the graph as it existed after the first (upto+1)
// operations in this replica's log. It creates a throwaway replica, replays the
// ops into it, and returns its graph snapshot — the same shape as GET /graph.
//
// Query param: upto (int, 0-based index into the op log). Out-of-bounds values
// are clamped to the last op.
func (h *Handler) handleReplay(c *gin.Context) {
	uptoStr := c.Query("upto")
	upto, err := strconv.Atoi(uptoStr)
	if err != nil || upto < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upto must be a non-negative integer"})
		return
	}

	ops := h.replica.OpLog()
	if len(ops) == 0 {
		temp := replica.New("replay", nil, nil)
		c.JSON(http.StatusOK, temp.Snapshot())
		return
	}
	if upto >= len(ops) {
		upto = len(ops) - 1
	}

	temp := replica.New("replay", nil, nil)
	for _, op := range ops[:upto+1] {
		temp.Ingest(op)
	}
	c.JSON(http.StatusOK, temp.Snapshot())
}
