.PHONY: build clean dev test web

# Build the web frontend and Go binary
build: web
	go build -o agent-viz ./cmd/agent-viz/

# Build web frontend only
web:
	cd web && npm install && npx vite build

# Run in dev mode (requires Go backend running separately)
dev:
	cd web && npx vite

# Run Go tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f agent-viz
	rm -rf internal/server/dist/
	mkdir -p internal/server/dist && echo "placeholder" > internal/server/dist/.gitkeep
