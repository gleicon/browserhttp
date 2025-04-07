.PHONY: all build test build-examples build-burl run-burl clean prepare-release

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

# Helper function to bump version
define bump_version
	@current=$$(git describe --tags --abbrev=0 2>/dev/null || echo 'v0.0.0'); \
	current=$${current#v}; \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	patch=$$(echo $$current | cut -d. -f3); \
	case "$1" in \
		major) echo "v$$(($$major + 1)).0.0";; \
		minor) echo "v$$major.$$(($$minor + 1)).0";; \
		patch) echo "v$$major.$$minor.$$(($$patch + 1))";; \
		*) echo "$$current";; \
	esac
endef

# Release preparation commands
prepare-release:
	@echo "Preparing release..."
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: There are uncommitted changes. Please commit or stash them first."; \
		exit 1; \
	fi
	@echo "Current version: $$(git describe --tags --abbrev=0 2>/dev/null || echo 'v0.0.0')"
	@echo "Select version bump type:"
	@echo "1) Major (breaking changes)"
	@echo "2) Minor (new features)"
	@echo "3) Patch (bug fixes)"
	@echo "4) Custom version"
	@read -p "Enter choice (1-4): " choice; \
	case "$$choice" in \
		1) version=$$(make -s bump_version major);; \
		2) version=$$(make -s bump_version minor);; \
		3) version=$$(make -s bump_version patch);; \
		4) read -p "Enter custom version (e.g., v0.1.0): " version;; \
		*) echo "Invalid choice"; exit 1;; \
	esac; \
	if [ -z "$$version" ]; then \
		echo "Error: Version cannot be empty"; \
		exit 1; \
	fi; \
	read -p "Enter release message: " message; \
	if [ -z "$$message" ]; then \
		echo "Error: Message cannot be empty"; \
		exit 1; \
	fi; \
	echo "Creating tag $$version with message: $$message"; \
	git tag -a $$version -m "$$message" && \
	git push origin $$version && \
	echo "Release $$version prepared successfully!"

# ==============================================================================
# Release Management
# ==============================================================================
#
# This project uses Semantic Versioning (SemVer) for version numbers:
#   MAJOR.MINOR.PATCH
#
# Version Bump Types:
#   1. Major (X.0.0): Breaking changes that require users to update their code
#   2. Minor (0.X.0): New features that are backwards compatible
#   3. Patch (0.0.X): Bug fixes that are backwards compatible
#
# Release Process:
#   1. Ensure all changes are committed
#   2. Run tests: make test
#   3. Prepare release: make prepare-release
#   4. Select version bump type
#   5. Enter release message
#   6. Tag is created and pushed automatically
#
# Using the Library:
#   Latest version:
#     go get github.com/gleicon/browserhttp
#
#   Specific version:
#     go get github.com/gleicon/browserhttp@v0.1.0
#
#   Update to latest:
#     go get -u github.com/gleicon/browserhttp
#
# Best Practices:
#   - Start with v0.x.x for initial development
#   - Move to v1.x.x when API is stable
#   - Always use semantic versioning
#   - Document breaking changes in release notes
#   - Keep go.mod file clean and up to date
#   - Use go mod tidy regularly
#
# ==============================================================================
