.PHONY: all build test build-examples build-burl run-burl clean

all: deps build build-examples

deps:
	@echo "Deps"
	go mod download
	go mod verify

build:
	@echo "Building browserhttp library..."
	go build .

test:
	@echo "Running tests..."
	go test .

build-examples:
	@echo "Building all examples..."
	go build -o bin/get examples/get.go
	go build -o bin/post examples/post.go
	go build -o bin/verbose examples/verbose.go
	go build -o bin/multitabs examples/multitabs.go

	go build -o bin/get_with_screenshot examples/get_with_screenshot.go
	go build -o bin/multitab_with_screenshots examples/multitab_with_screenshots.go
	go build -o bin/post_with_screenshot examples/post_with_screenshot.go


build-burl:
	@echo "Building burl CLI..."
	go build -o bin/burl examples/burl/burl.go

run-burl:
	@echo "Running burl CLI..."
	go run examples/burl/burl.go

clean:
	rm -rf bin
	rm -f browserhttp.test
