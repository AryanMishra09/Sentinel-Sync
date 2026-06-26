// Package api is the client-facing REST layer for a replica.
//
// Like SentinelCache, the split is deliberate: REST (Gin) faces clients and the
// dashboard because it is human-readable and curl-friendly; replica-to-replica
// traffic (added in Phase 4) will use a separate transport. Gin never sees a
// replication message.
package api

import (
	"errors"
	"net/http"

	"github.com/aryan-mishra/sentinel-sync/internal/graph"
	"github.com/gin-gonic/gin"
)

// Handler wires HTTP requests to the graph engine.
type Handler struct {
	replicaID string
	graph     *graph.Graph
}

// NewHandler builds a handler bound to one replica's graph.
func NewHandler(replicaID string, g *graph.Graph) *Handler {
	return &Handler{replicaID: replicaID, graph: g}
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
	n, err := h.graph.CreateNode(req.ID, req.Title, req.X, req.Y)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, n)
}

func (h *Handler) handleRenameNode(c *gin.Context) {
	var req renameNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := h.graph.RenameNode(c.Param("id"), req.Title)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) handleMoveNode(c *gin.Context) {
	var req moveNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := h.graph.MoveNode(c.Param("id"), req.X, req.Y)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) handleDeleteNode(c *gin.Context) {
	if err := h.graph.DeleteNode(c.Param("id")); err != nil {
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
	e, err := h.graph.CreateEdge(req.ID, req.Source, req.Target)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, e)
}

func (h *Handler) handleDeleteEdge(c *gin.Context) {
	if err := h.graph.DeleteEdge(c.Param("id")); err != nil {
		h.writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) handleGetGraph(c *gin.Context) {
	c.JSON(http.StatusOK, h.graph.Snapshot())
}

func (h *Handler) handleStatus(c *gin.Context) {
	nodes, edges := h.graph.Counts()
	c.JSON(http.StatusOK, gin.H{
		"replicaId": h.replicaID,
		"nodes":     nodes,
		"edges":     edges,
	})
}

// writeErr maps engine sentinel errors to HTTP status codes.
func (h *Handler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, graph.ErrNodeNotFound), errors.Is(err, graph.ErrEdgeNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrNodeExists), errors.Is(err, graph.ErrEdgeExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, graph.ErrEndpointMissing):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
