package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// --- /sim request bodies ---------------------------------------------------

type setLatencyReq struct {
	Ms int64 `json:"ms"`
}

type setLossReq struct {
	Rate float64 `json:"rate"`
}

// --- /sim handlers ---------------------------------------------------------

// POST /sim/latency  {"ms":200}   — set outgoing message delay
func (h *Handler) handleSetLatency(c *gin.Context) {
	var req setLatencyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Ms < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ms must be >= 0"})
		return
	}
	h.chaos.SetLatency(time.Duration(req.Ms) * time.Millisecond)
	c.JSON(http.StatusOK, h.chaos.Snapshot())
}

// POST /sim/loss  {"rate":0.3}   — set outgoing packet loss probability [0,1]
func (h *Handler) handleSetLoss(c *gin.Context) {
	var req setLossReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Rate < 0 || req.Rate > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rate must be between 0.0 and 1.0"})
		return
	}
	h.chaos.SetLossRate(req.Rate)
	c.JSON(http.StatusOK, h.chaos.Snapshot())
}

// POST /sim/isolate   — soft-partition this replica (both directions blocked)
func (h *Handler) handleIsolate(c *gin.Context) {
	h.chaos.SetIsolated(true)
	c.JSON(http.StatusOK, h.chaos.Snapshot())
}

// POST /sim/recover   — lift soft-partition; next anti-entropy tick catches up
func (h *Handler) handleRecover(c *gin.Context) {
	h.chaos.SetIsolated(false)
	c.JSON(http.StatusOK, h.chaos.Snapshot())
}
