.PHONY: all test test-verbose build run clean deps gen-summary

# Build the application
build:
	go build -o bin/comemo .

# Run tests
test:
	go test ./internal/...

# Run tests with verbose output
test-verbose:
	go test -v ./internal/...

# Run the application
run:
	go run main.go

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
