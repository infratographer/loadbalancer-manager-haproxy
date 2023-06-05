all: lint test
PHONY: test unit-test coverage lint golint clean build vendor
GOOS=linux
APP_NAME=loadbalancer-manager-haproxy

help: Makefile ## Print help
	@grep -h "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/:.*##/#/' | column -c 2 -t -s#

test: | lint unit-test

unit-test: ## Run unit tests
	@echo Running unit tests...
	@go test -cover -short -tags testtools ./...

coverage: ## Run unit tests with coverage
	@echo Generating coverage report...
	@go test ./... -race -coverprofile=coverage.out -covermode=atomic -tags testtools -p 1
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out

lint: golint ## Runs linting

golint: | vendor
	@echo Linting Go files...
	@golangci-lint run --build-tags "-tags testtools"

build: ## Build the binary
	@go mod download
	@CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o bin/${APP_NAME}

clean: ## Clean up all the things
	@echo Cleaning...
	@rm -f bin/${APP_NAME}
	@rm -rf ./dist/
	@rm -rf coverage.out
	@go clean -testcache

vendor: ## Vendors dependencies
	@go mod download
	@go mod tidy
