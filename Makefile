ELVISH_MAKE_BIN ?= $(shell go env GOPATH)/bin/elvish
ELVISH_PLUGIN_SUPPORT ?= 0

# Treat 0 as false and everything else as true (consistent with CGO_ENABLED).
ifeq ($(ELVISH_PLUGIN_SUPPORT), 0)
    REPRODUCIBLE := true
else
    REPRODUCIBLE := false
endif

default: test get

get:
	export CGO_ENABLED=$(ELVISH_PLUGIN_SUPPORT); \
	if go env GOOS GOARCH | egrep -qx '(windows .*|linux (amd64|arm64))'; then \
		export GOFLAGS=-buildmode=pie; \
	fi; \
	mkdir -p $(shell dirname $(ELVISH_MAKE_BIN))
	go build -o $(ELVISH_MAKE_BIN) -trimpath -ldflags \
		"-X src.elv.sh/pkg/buildinfo.VersionSuffix=-dev.$$(git rev-parse HEAD)$$(git diff HEAD --quiet || printf +%s `uname -n`) \
		 -X src.elv.sh/pkg/buildinfo.Reproducible=$(REPRODUCIBLE)" ./cmd/elvish

generate:
	go generate ./...

# Run unit tests, with race detection if the platform supports it.
test:
	go test $(shell ./tools/run-race.sh) ./...
	cd website; go test $(shell ./tools/run-race.sh) ./...

# Generate a basic test coverage report, and open it in the browser. See also
# https://apps.codecov.io/gh/elves/elvish/.
cover:
	go test -coverprofile=cover -coverpkg=./pkg/... ./pkg/...
	./tools/prune-cover.sh .codecov.yml cover
	go tool cover -html=cover
	go tool cover -func=cover | tail -1 | awk '{ print "Overall coverage:", $$NF }'

# Ensure the style of Go and Markdown source files is consistent.
style:
	find . -name '*.go' | xargs goimports -w
	find . -name '*.go' | xargs gofmt -s -w
	find . -name '*.md' | xargs prettier --write

# Check if the style of the Go and Markdown files is correct without modifying
# those files.
checkstyle: checkstyle-go checkstyle-md

checkstyle-go:
	./tools/checkstyle-go.sh

checkstyle-md:
	./tools/checkstyle-md.sh

lint:
	./tools/lint.sh

codespell:
	codespell --skip .git

.SILENT: checkstyle-go checkstyle-md lint
.PHONY: default get generate test style checkstyle checkstyle-go checkstyle-md lint cover
