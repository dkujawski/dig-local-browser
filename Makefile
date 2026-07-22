BINARY := chromecarve
PACKAGE := ./cmd/chromecarve
BUILD_DIR := build

.PHONY: all deps fmt-check test test-race vet validate build clean

all: validate build

deps:
	go mod verify
	go mod tidy -diff
	go list -deps ./... >/dev/null

fmt-check:
	test -z "$$(gofmt -l .)"

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

validate: deps fmt-check test vet

build:
	mkdir -p $(BUILD_DIR)
	go build -trimpath -o $(BUILD_DIR)/$(BINARY) $(PACKAGE)

clean:
	rm -rf -- $(BUILD_DIR) dist
