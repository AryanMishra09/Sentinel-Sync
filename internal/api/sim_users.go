package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type startSimReq struct {
	Users     int     `json:"users"`
	OpsPerSec float64 `json:"opsPerSec"`
}

// POST /sim/users/start  {"users":10,"opsPerSec":5.0}
func (h *Handler) handleSimStart(c *gin.Context) {
	var req startSimReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Users <= 0 || req.OpsPerSec <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "users and opsPerSec must be > 0"})
		return
	}
	h.simulator.Start(req.Users, req.OpsPerSec)
	c.JSON(http.StatusOK, h.simulator.Stats())
}

// POST /sim/users/stop
func (h *Handler) handleSimStop(c *gin.Context) {
	h.simulator.Stop()
	c.JSON(http.StatusOK, h.simulator.Stats())
}

// GET /sim/users/stats
func (h *Handler) handleSimStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.simulator.Stats())
}
