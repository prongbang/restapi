all: test build ## Run the tests and build the binary.

build: ## Build the binary.
	go build -ldflags "-X github.com/prongbang/restapi/cmd.Version=`git rev-parse HEAD`"

lint: ## Lint the code
	golint `go list ./... | grep -v /vendor/`

test: ## Run tests.
	go test -v `go list ./... | grep -v /vendor/`

deps: ## Install dependencies.
	@go get -u github.com/golang/lint/golint
	@go get -u github.com/Masterminds/glide && glide install