.PHONY: fmt fmt-check vet vet-windows build-windows test ci-local hooks

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "needs gofmt:"; \
		echo "$$out"; \
		exit 1; \
	fi

vet:
	go vet ./...

vet-windows:
	GOMAXPROCS=1 GOOS=windows go vet ./...

build-windows:
	GOMAXPROCS=1 GOOS=windows go build -o /dev/null ./...

test:
	go test ./components/... ./cmd/... ./tools/...

ci-local: fmt-check vet vet-windows build-windows test

hooks:
	@git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath=.githooks); pre-commit runs 'make ci-local'"
