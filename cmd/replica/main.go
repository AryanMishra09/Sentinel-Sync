// Command replica is the SentinelSync node binary.
//
// Phase 1: it runs a single, standalone graph engine behind a REST API. There
// is no networking between replicas yet — each process is an island. Config
// comes from environment variables so Docker Compose can spin up replica-a/b/c
// from one image (the pattern carries over from SentinelCache).
package main

import (
	"log"
	"os"

	"github.com/aryan-mishra/sentinel-sync/internal/api"
	"github.com/aryan-mishra/sentinel-sync/internal/graph"
	"github.com/gin-gonic/gin"
)

func main() {
	replicaID := env("REPLICA_ID", "replica-a")
	restAddr := env("REST_ADDR", ":8080")

	g := graph.New()
	h := api.NewHandler(replicaID, g)

	r := gin.Default()
	h.RegisterRoutes(r)

	log.Printf("[%s] SentinelSync replica listening on %s (Phase 1: single replica, no sync)", replicaID, restAddr)
	if err := r.Run(restAddr); err != nil {
		log.Fatalf("[%s] server error: %v", replicaID, err)
	}
}

// env reads an environment variable with a fallback default.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
