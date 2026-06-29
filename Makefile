.PHONY: build run test tidy clean docker-up docker-down docker-logs frontend-install frontend-dev frontend-build

# Compile the replica binary to bin/replica.
build:
	go build -o bin/replica ./cmd/replica

# Run a single replica locally for quick testing (Phase 1).
run:
	go run ./cmd/replica

# Run all tests with the race detector — important for the graph engine's lock.
test:
	go test -race ./...

# Sync dependencies.
tidy:
	go mod tidy

# Build images and start the 3-replica cluster (replica-a/b/c).
docker-up:
	docker compose up --build -d

# Stop and remove the cluster.
docker-down:
	docker compose down

# Tail logs from all replicas.
docker-logs:
	docker compose logs -f

# Remove build artifacts.
clean:
	rm -rf bin/ frontend/dist frontend/node_modules

# ── Frontend (Phase 7) ────────────────────────────────────────────────────

# Install frontend npm dependencies (run once after cloning).
frontend-install:
	cd frontend && npm install

# Start the Vite dev server on :3000 (hot-reload; replicas must be running).
frontend-dev:
	cd frontend && npm run dev

# Build the production bundle to frontend/dist/.
frontend-build:
	cd frontend && npm run build
