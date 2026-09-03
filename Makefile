.PHONY: test cover crap mutate quality install-tools

install-tools:
	go install github.com/padiazg/go-crap@latest
	go install github.com/go-gremlins/gremlins/cmd/gremlins@latest

test:
	go test -v ./...

cover:
	go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out

crap:
	@which go-crap >/dev/null 2>&1 || $(MAKE) install-tools
	go-crap scan --threshold 8 --fail-above

mutate:
	@which gremlins >/dev/null 2>&1 || $(MAKE) install-tools
	gremlins unleash -o gremlins-report.json

quality: cover mutate
	@which go-crap >/dev/null 2>&1 || $(MAKE) install-tools
	go-crap scan --coverage-profile coverage.out --mutation-report gremlins-report.json --threshold 8 --fail-above
