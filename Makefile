.PHONY: build run test tidy clean

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

# Remove build artifacts.
clean:
	rm -rf bin/
