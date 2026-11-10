.PHONY: all run/fix run/tidy fmt/sources fmt/imports test/unit check/vet check/lint

ifeq ($(shell command -v gotestsum 2> /dev/null),)
    TEST_RUNNER := go test
else
    TEST_RUNNER := gotestsum --
endif

all: run/fix run/tidy fmt/sources fmt/imports test/unit check/vet check/lint

test/unit:
	@echo "clean cache"
	@go clean -testcache
	@echo "run tests"
	@$(TEST_RUNNER) -race -cover -coverprofile=coverage.out ./...
	@CURRENT_COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); echo "Code coverage: $$CURRENT_COVERAGE%"
	@rm -f coverage.out

check/lint:
	@if [ -x "$$(command -v golangci-lint)" ]; then echo "run golangci-lint..." ; golangci-lint run ./... ; else echo "golangci-lint not found" ; fi

check/tidy:
	@echo "check go mod tidy"
	@go mod tidy && git diff --exit-code -- go.mod go.sum

check/vuln:
	@if [ -x "$$(command -v govulncheck)" ]; then echo "run govulncheck..." ; govulncheck -show verbose ./... ; else echo "govulncheck not found" ; fi

check/vet:
	@echo "run go vet"
	@go vet ./...

run/fix:
	@echo "run go fix"
	@go fix ./...

run/tidy:
	@echo "run go mod tidy"
	@go mod tidy

fmt/imports:
	@if [ -x "$$(command -v gci)" ]; then echo "run gci" ; gci write --skip-generated -s standard -s default -s "prefix(github.com/wmentor/go-magnetar)" . ; else echo "gci not found" ; fi

fmt/sources:
	@if [ -x "$$(command -v gofumpt)" ]; then echo "run gofumpt" ; gofumpt -l -w . ; else echo "gofumpt not found" ; fi
