BINARY := go-mdtopdf-helper
TOMD_BINARY := go-tomd
BIN_DIR := bin
CMD := ./cmd/go-mdtopdf-helper
TOMD_CMD := ./cmd/go-tomd

.PHONY: all build run run-tomd test lint format clean release-version

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

# Bump the latest vMAJOR.MINOR.PATCH tag and push the new tag, which triggers the release workflow.
#   make release-version              patch bump (v1.0.0 -> v1.0.1)
#   make release-version BUMP=minor   minor bump (v1.0.1 -> v1.1.0)
#   make release-version BUMP=major   major bump (v1.1.0 -> v2.0.0)
#   make release-version DRY_RUN=1    show the next version without tagging or pushing
BUMP ?= patch
release-version:
	@set -e; \
	case "$(BUMP)" in patch|minor|major) ;; *) echo "BUMP must be patch, minor or major (got '$(BUMP)')" >&2; exit 1;; esac; \
	git fetch --tags --quiet; \
	latest=$$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -n1 || true); \
	latest=$${latest:-v0.0.0}; \
	ver=$${latest#v}; major=$${ver%%.*}; rest=$${ver#*.}; minor=$${rest%%.*}; patch=$${rest#*.}; \
	case "$(BUMP)" in \
		major) major=$$((major + 1)); minor=0; patch=0;; \
		minor) minor=$$((minor + 1)); patch=0;; \
		patch) patch=$$((patch + 1));; \
	esac; \
	next="v$$major.$$minor.$$patch"; \
	echo "Latest tag: $$latest -> next: $$next"; \
	if [ -n "$(DRY_RUN)" ]; then echo "Dry run: nothing tagged or pushed"; exit 0; fi; \
	if [ -n "$$(git status --porcelain)" ]; then echo "Working tree is not clean; commit or stash first" >&2; exit 1; fi; \
	if [ -n "$$(git rev-list '@{u}..HEAD' 2>/dev/null)" ]; then echo "HEAD has unpushed commits; push them first" >&2; exit 1; fi; \
	git tag -a "$$next" -m "Release $$next"; \
	git push origin "$$next"; \
	echo "Pushed $$next"
