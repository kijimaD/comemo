.PHONY: all test test-verbose test-race build run clean deps gen-summary fmt lint vet tools-install goimports check

# Build the application
build:
	go build -o bin/comemo .

# Run tests
test:
	go test -v -cover -race ./internal/...

# Run the application
run:
	go run main.go

# Generate summary (existing functionality)
gen-summary:
	./scripts/gen-summary.sh

# Format imports using goimports
fmt:
	goimports -w ./internal/ ./cli/ main.go

# Run go vet
vet:
	go vet ./internal/... ./cli/... .

# Run basic linting (vet + fmt check)
lint: vet
	@echo "Checking formatting..."
	@gofmt -l ./internal/ ./cli/ main.go | grep -E '\.go$$' && echo "Code needs formatting. Run 'make fmt'" && exit 1 || echo "Code is properly formatted"
	@echo "Checking imports..."
	@goimports -l ./internal/ ./cli/ main.go | grep -E '\.go$$' && echo "Imports need formatting. Run 'make goimports'" && exit 1 || echo "Imports are properly formatted"

# Install required development tools
tools-install:
	go install golang.org/x/tools/cmd/goimports@latest

# Run comprehensive checks
check: fmt vet test
