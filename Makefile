.PHONY: fmt fmt-check vet test ci-local

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

test:
	go test ./components/... ./cmd/... ./tools/...

ci-local: fmt-check vet test
