.PHONY: all test test-verbose build run clean deps gen-summary

# Build the application
build:
	go build -o bin/comemo pkg/cmd/comemo/main.go

# Run tests
test:
	go test ./internal/... ./pkg/...

# Run tests with verbose output  
test-verbose:
	go test -v ./internal/... ./pkg/...

# Run the application
run:
	go run pkg/cmd/comemo/main.go

# Clean build artifacts and test cache
clean:
	rm -rf bin/
	go clean -testcache

# Install dependencies
deps:
	go mod tidy

# Generate summary (existing functionality)
gen-summary:
	./scripts/gen-summary.sh

all: build
