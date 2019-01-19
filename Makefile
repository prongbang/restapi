all: test build ## Run the tests and build the binary.

build: ## Build the binary.
	go get -u github.com/Masterminds/glide && glide install
	go build

lint: ## Lint the code
	golint `go list ./... | grep -v /vendor/`

test: ## Run tests.
	go test -v `go list ./... | grep -v /vendor/`

deps: ## Install dependencies.
	@go get -u github.com/golang/lint/golint
	@go get -u github.com/Masterminds/glide && glide install