# ==========================================
# FH6 Telemetry - Makefile
# ==========================================
# Targets:
#   make dev    - Build frontend + run Go server (dev mode)
#   make build  - Build frontend only (SvelteKit)
#   make server - Build Go binary (local linux)
#   make run    - Build frontend + Go binary, then run
#   make release - Build all release binaries (cross-compile)
#   make clean  - Remove build artifacts
# ==========================================

BINARY_NAME = fh6-telemetry
GO_DIR      = go-server
DIST_DIR    = dist
BUILD_DIR   = build

.PHONY: dev build server run release clean

## Build frontend then start Go server (hottest dev loop)
dev: build
	@echo "🚀 Starting server..."
	@cd $(GO_DIR) && go run .

## Build SvelteKit frontend only
build:
	@echo "📦 Building frontend..."
	@npm run build
	@echo "✅ Frontend built → $(BUILD_DIR)/"

## Compile Go binary for local Linux (fast, no cross-compile)
server:
	@echo "⚙️  Compiling Go server..."
	@cd $(GO_DIR) && go build -ldflags="-X 'main.IsMinimal=false'" -o $(BINARY_NAME)
	@echo "✅ Binary: $(GO_DIR)/$(BINARY_NAME)"

## Build frontend + Go binary, then run
run: build server
	@echo "🚀 Running server..."
	@cd $(GO_DIR) && ./$(BINARY_NAME)

## Build all cross-compiled release binaries (uses the release script)
release: build
	@echo "🏁 Building all release targets..."
	@go run scripts/build_release.go
	@echo "✅ Release binaries in $(DIST_DIR)/"

## Remove all build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR) .svelte-kit $(GO_DIR)/embed_build $(GO_DIR)/embed_ui $(GO_DIR)/$(BINARY_NAME)
	@echo "✅ Clean done"
