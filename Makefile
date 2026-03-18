.PHONY: build test lint run clean fmt fmt-check docker

BINARY := instagram-mcp
BUILD_DIR := ./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/instagram-mcp

test:
	go test -race -count=1 -cover ./...

lint:
	golangci-lint run ./...

run: build
	$(BUILD_DIR)/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)

fmt:
	gofmt -s -w .
	goimports -w -local github.com/vikrant/instagram-mcp .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

docker:
	docker build -t $(BINARY):latest .
