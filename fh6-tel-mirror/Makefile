# ==========================================
# FH6 Telemetry Mirror - Makefile
# ==========================================

BINARY_NAME = fh6-tel-mirror
DIST_DIR    = dist

.PHONY: all build clean release linux windows darwin

all: clean build

## Compile local OS binary
build:
	@echo "⚙️  Compiling local binary..."
	@go build -o $(BINARY_NAME) .
	@echo "✅ Built: ./$(BINARY_NAME)"

## Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BINARY_NAME) $(BINARY_NAME).exe $(DIST_DIR)
	@echo "✅ Clean completed"

## Multi-platform release compiler
release: clean linux windows darwin
	@echo "🏁 All multi-release binaries built in $(DIST_DIR)/"

## Build targets for Linux
linux:
	@echo "📦 Building Linux releases..."
	@mkdir -p $(DIST_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "✅ Linux builds complete."

## Build targets for Windows
windows:
	@echo "📦 Building Windows releases..."
	@mkdir -p $(DIST_DIR)
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe .
	@echo "✅ Windows builds complete."

## Build targets for macOS (Darwin)
darwin:
	@echo "📦 Building macOS releases..."
	@mkdir -p $(DIST_DIR)
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "✅ macOS builds complete."
