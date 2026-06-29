// Command replica is the SentinelSync node binary.
//
// Phase 2: every process is an equal, independent replica. It builds its own
// graph and knows its peers' addresses (from the PEERS env var), but replicas do
// NOT talk to each other yet — there is no synchronization. Config comes from
// environment variables so Docker Compose can spin up replica-a/b/c from one
// image.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aryan-mishra/sentinel-sync/internal/api"
	"github.com/aryan-mishra/sentinel-sync/internal/replica"
	"github.com/aryan-mishra/sentinel-sync/internal/simulation"
	"github.com/aryan-mishra/sentinel-sync/internal/transport"
	"github.com/gin-gonic/gin"
)

func main() {
	replicaID := env("REPLICA_ID", "replica-a")
	restAddr := env("REST_ADDR", ":8080")
	peers := parsePeers(env("PEERS", ""))

	chaos := simulation.NewChaos()
	r := replica.New(replicaID, peers, nil) // nil → real wall clock for the HLC
	sim := simulation.NewSimulator(r)
	h := api.NewHandler(r, chaos, sim)

	// Replica-to-replica WebSocket transport: broadcast + anti-entropy.
	mgr := transport.NewManager(r, chaos)
	go mgr.Start(context.Background())

	engine := gin.Default()

	// CORS — allows the dashboard (served on :3000) to call the replica REST APIs.
	engine.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	h.RegisterRoutes(engine)
	engine.GET("/ws", gin.WrapF(mgr.HandleWS)) // peer connections land here

	log.Printf("[%s] SentinelSync replica on %s — %d peer(s), full stack active (Phase 7 — dashboard)",
		replicaID, restAddr, len(peers))
	if err := engine.Run(restAddr); err != nil {
		log.Fatalf("[%s] server error: %v", replicaID, err)
	}
}

// parsePeers turns "replica-b=http://replica-b:8080,replica-c=http://replica-c:8080"
// into a peer list. Entries without an "=" are skipped. An empty string yields no
// peers (single-replica mode, as in Phase 1).
func parsePeers(raw string) []replica.Peer {
	var peers []replica.Peer
	for entry := range strings.SplitSeq(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		id, addr, ok := strings.Cut(entry, "=")
		if !ok {
			log.Printf("ignoring malformed PEERS entry %q (want id=address)", entry)
			continue
		}
		peers = append(peers, replica.Peer{ID: strings.TrimSpace(id), Address: strings.TrimSpace(addr)})
	}
	return peers
}

// env reads an environment variable with a fallback default.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
