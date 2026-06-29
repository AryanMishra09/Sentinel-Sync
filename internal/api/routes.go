package api

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the Phase 1 REST API.
//
// Resource-oriented layout: /node and /edge are the two mutable resources,
// /graph is the read model, /health and /status are operational. Later phases
// add /sim and /network controls under the same router.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/status", h.handleStatus)
	r.GET("/graph", h.handleGetGraph)

	r.POST("/node", h.handleCreateNode)
	r.PATCH("/node/:id/title", h.handleRenameNode)
	r.PATCH("/node/:id/position", h.handleMoveNode)
	r.DELETE("/node/:id", h.handleDeleteNode)

	r.POST("/edge", h.handleCreateEdge)
	r.DELETE("/edge/:id", h.handleDeleteEdge)

	// Phase 5 — network fault injection
	r.POST("/sim/latency", h.handleSetLatency)
	r.POST("/sim/loss", h.handleSetLoss)
	r.POST("/sim/isolate", h.handleIsolate)
	r.POST("/sim/recover", h.handleRecover)

	// Phase 6 — simulated users
	r.POST("/sim/users/start", h.handleSimStart)
	r.POST("/sim/users/stop", h.handleSimStop)
	r.GET("/sim/users/stats", h.handleSimStats)
}
