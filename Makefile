.PHONY: fmt golangci-lint

all: fmt golangci-lint

fmt:
	gofumpt -l -w .

golangci-lint:
	golangci-lint run
