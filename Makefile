.PHONY: all build-lib test-lib build-examples build-burl run-burl clean

all: deps build-lib build-examples

deps:
	@echo "Deps"
	go mod download
	go mod verify

build-lib:
	@echo "Building browserhttp library..."
	go build ./browserhttp

test-lib:
	@echo "Running tests..."
	go test ./browserhttp

build-examples:
	@echo "Building all examples..."
	go build -o bin/get examples/get.go
	go build -o bin/post examples/post.go
	go build -o bin/verbose examples/verbose.go

build-burl:
	@echo "Building burl CLI..."
	go build -o bin/burl examples/burl/burl.go

run-burl:
	@echo "Running burl CLI..."
	go run examples/burl/burl.go

clean:
	rm -rf bin
	rm -f browserhttp.test
