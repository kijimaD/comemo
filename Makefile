.DEFAULT_GOAL := help

.PHONY: build
build: ## Build the application
	go build -o bin/comemo .

.PHONY: test
test: ## Run tests with coverage and race detection
	go test -v -cover -race ./internal/...

.PHONY: run
run: ## Run the application
	go run main.go

.PHONY: gen-summary
gen-summary: ## Generate summary (existing functionality)
	./scripts/gen-summary.sh

.PHONY: fmt
fmt: ## Format code using goimports
	goimports -w ./internal/ ./cli/ main.go

.PHONY: lint
lint: ## Run basic linting (vet + fmt check + golangci-lint)
	go vet ./internal/... ./cli/... .
	@echo "Checking formatting..."
	@gofmt -l ./internal/ ./cli/ main.go | grep -E '\.go$$' && echo "Code needs formatting. Run 'make fmt'" && exit 1 || echo "Code is properly formatted"
	@echo "Checking imports..."
	@goimports -l ./internal/ ./cli/ main.go | grep -E '\.go$$' && echo "Imports need formatting. Run 'make goimports'" && exit 1 || echo "Imports are properly formatted"
	@echo "Running golangci-lint..."
	@golangci-lint run -v ./internal/... ./cli/...

.PHONY: tools-install
tools-install: ## Install development tools
	@go install golang.org/x/tools/cmd/goimports@latest
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin)

.PHONY: check
check: build test fmt lint ## Run comprehensive code quality checks

.PHONY: help
help: ## ヘルプを表示する
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
