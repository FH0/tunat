.PHONY: fmt golangci-lint

all: fmt golangci-lint test-linux

fmt:
	gofumpt -l -w .

golangci-lint:
	golangci-lint run

test-linux:
	go test -race -cover ./...
