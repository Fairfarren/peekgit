.DEFAULT_GOAL := test
.PHONY: test cover check-coverage crap mutate quality install-tools format-check check gate-test

QUALITY_BIN := $(CURDIR)/.quality-bin
GREMLINS_VERSION := v0.6.0
GO_CRAP_VERSION := v0.5.1
GREMLINS := $(QUALITY_BIN)/gremlins-$(GREMLINS_VERSION)
GO_CRAP := $(QUALITY_BIN)/go-crap-$(GO_CRAP_VERSION)

$(GREMLINS):
	mkdir -p "$(QUALITY_BIN)"
	GOBIN="$(QUALITY_BIN)" go install github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)
	mv "$(QUALITY_BIN)/gremlins" "$@"

$(GO_CRAP):
	mkdir -p "$(QUALITY_BIN)"
	GOBIN="$(QUALITY_BIN)" go install github.com/padiazg/go-crap@$(GO_CRAP_VERSION)
	mv "$(QUALITY_BIN)/go-crap" "$@"

install-tools: $(GREMLINS) $(GO_CRAP)

format-check:
	@files=$$(gofmt -l cmd internal) || exit $$?; \
		test -z "$$files" || { printf '%s\n' "$$files"; exit 1; }

test:
	go test -count=1 -timeout=90s ./...

check: format-check test
	go vet ./...
	go build ./...

cover:
	go test -count=1 -timeout=90s -coverprofile=coverage.out -covermode=atomic ./...
	$(MAKE) check-coverage

check-coverage:
	awk -f scripts/check-coverage.awk coverage.out

crap: cover $(GO_CRAP)
	"$(GO_CRAP)" scan --coverage-profile coverage.out --threshold 8 --fail-above

mutate: $(GREMLINS)
	bash scripts/mutate.sh "$(GREMLINS)" "$(CURDIR)"

gate-test: $(GREMLINS)
	bash scripts/test-quality.sh "$(GREMLINS)"

quality:
	$(MAKE) check
	$(MAKE) gate-test
	$(MAKE) crap
	$(MAKE) mutate
