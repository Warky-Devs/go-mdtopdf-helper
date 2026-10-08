BINARY := go-mdtopdf-helper
TOMD_BINARY := go-tomd
BIN_DIR := bin
CMD := ./cmd/go-mdtopdf-helper
TOMD_CMD := ./cmd/go-tomd

.PHONY: all build run run-tomd test lint format clean

all: format lint test build

build:
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)
	go build -o $(BIN_DIR)/$(TOMD_BINARY) $(TOMD_CMD)

run:
	go run $(CMD) $(ARGS)

run-tomd:
	go run $(TOMD_CMD) $(ARGS)

test:
	go test ./...

lint:
	golangci-lint run ./...

format:
	gofmt -s -w .

clean:
	rm -rf $(BIN_DIR)
