// Package api is the client-facing REST layer for a replica.
//
// REST (Gin) faces clients and the dashboard because it is human-readable and
// curl-friendly; replica-to-replica traffic (Phase 4) will use a separate
// transport. Each handler maps to one CRDT operation on the replica.
package api

import (
	"errors"
	"net/http"

	"github.com/aryan-mishra/sentinel-sync/internal/graph"
	"github.com/aryan-mishra/sentinel-sync/internal/replica"
	"github.com/aryan-mishra/sentinel-sync/internal/simulation"
	"github.com/gin-gonic/gin"
)

// Handler wires HTTP requests to one replica.
type Handler struct {
	replica *replica.Replica
	chaos   *simulation.Chaos
}

// NewHandler builds a handler bound to a replica and its chaos injector.
func NewHandler(r *replica.Replica, chaos *simulation.Chaos) *Handler {
	return &Handler{replica: r, chaos: chaos}
}

// --- request bodies --------------------------------------------------------

type createNodeReq struct {
	ID    string  `json:"id" binding:"required"`
	Title string  `json:"title"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type renameNodeReq struct {
	Title string `json:"title" binding:"required"`
}

type moveNodeReq struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type createEdgeReq struct {
	ID     string `json:"id" binding:"required"`
	Source string `json:"source" binding:"required"`
	Target string `json:"target" binding:"required"`
}

// --- handlers --------------------------------------------------------------

func (h *Handler) handleCreateNode(c *gin.Context) {
	var req createNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.replica.CreateNode(req.ID, req.Title, req.X, req.Y); err != nil {
		h.writeErr(c, err)
		return
	}
	n, _ := h.replica.Node(req.ID)
	c.JSON(http.StatusCreated, n)
}

func (h *Handler) handleRenameNode(c *gin.Context) {
	var req renameNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := c.Param("id")
	if _, err := h.replica.RenameNode(id, req.Title); err != nil {
		h.writeErr(c, err)
		return
	}
	n, _ := h.replica.Node(id)
	c.JSON(http.StatusOK, n)
}

func (h *Handler) handleMoveNode(c *gin.Context) {
	var req moveNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := c.Param("id")
	if _, err := h.replica.MoveNode(id, req.X, req.Y); err != nil {
		h.writeErr(c, err)
		return
	}
	n, _ := h.replica.Node(id)
	c.JSON(http.StatusOK, n)
}

func (h *Handler) handleDeleteNode(c *gin.Context) {
	if _, err := h.replica.DeleteNode(c.Param("id")); err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) handleCreateEdge(c *gin.Context) {
	var req createEdgeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Note: no endpoint validation. A dangling edge is allowed by the CRDT and
	// filtered at materialization, so the echoed edge may not appear in /graph
	// until both endpoints exist.
	if _, err := h.replica.CreateEdge(req.ID, req.Source, req.Target); err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": req.ID, "source": req.Source, "target": req.Target})
}

func (h *Handler) handleDeleteEdge(c *gin.Context) {
	if _, err := h.replica.DeleteEdge(c.Param("id")); err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) handleGetGraph(c *gin.Context) {
	c.JSON(http.StatusOK, h.replica.Snapshot())
}

func (h *Handler) handleStatus(c *gin.Context) {
	nodes, edges := h.replica.Counts()
	c.JSON(http.StatusOK, gin.H{
		"replicaId":   h.replica.ID,
		"peers":       h.replica.Peers,
		"nodes":       nodes,
		"edges":       edges,
		"vectorClock": h.replica.Clock(),
		"opLogLen":    h.replica.OpLogLen(),
		"tombstones":  h.replica.TombstoneCount(),
		"stateHash":   h.replica.Hash(),
		"chaos":       h.chaos.Snapshot(),
	})
}

// writeErr maps engine sentinel errors to HTTP status codes.
func (h *Handler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, graph.ErrNodeNotFound), errors.Is(err, graph.ErrEdgeNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrNodeExists), errors.Is(err, graph.ErrEdgeExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
