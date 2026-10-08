BINARY := go-mdtopdf-helper
BIN_DIR := bin
CMD := ./cmd/go-mdtopdf-helper

.PHONY: all build run test lint format clean

all: format lint test build

build:
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

run:
	go run $(CMD) $(ARGS)

test:
	go test ./...

lint:
	golangci-lint run ./...

format:
	gofmt -s -w .

clean:
	rm -rf $(BIN_DIR)
