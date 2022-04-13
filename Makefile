anxcloud-cloud-controller-manager:
	go build

test: hack
	hack/ginkgo run -p              \
	    -timeout 0                  \
	    -race                       \
	    -coverprofile coverage.out  \
	    --keep-going                \
	    ./anx/...
	go tool cover -html=coverage.out -o coverage.html

run: anxcloud-cloud-controller-manager
	hack/anxkube-dev-run

debug:
	hack/anxkube-dev-run debug

hack:
	cd hack && go build -o . github.com/client9/misspell/cmd/misspell
	cd hack && go build -o . github.com/golangci/golangci-lint/cmd/golangci-lint
	cd hack && go build -o . github.com/onsi/ginkgo/v2/ginkgo

go-lint: hack
	@echo "==> Checking source code against linters..."
	@hack/golangci-lint run ./...
	@hack/golangci-lint run --build-tags integration ./...

docs-lint: hack
	@echo "==> Checking docs against linters..."
	@hack/misspell -error -source=text docs/ || (echo; \
		echo "Unexpected misspelling found in docs files."; \
		echo "To automatically fix the misspelling, run 'make docs-lint-fix' and commit the changes."; \
		exit 1)
	@docker run -v $(PWD):/markdown 06kellyjac/markdownlint-cli docs/ || (echo; \
		echo "Unexpected issues found in docs Markdown files."; \
		echo "To apply any automatic fixes, run 'make docs-lint-fix' and commit the changes."; \
		exit 1)

docs-lint-fix: tools
	@echo "==> Applying automatic docs linter fixes..."
	@hack/misspell -w -source=text docs/
	@docker run -v $(PWD):/markdown 06kellyjac/markdownlint-cli --fix docs/

.PHONY: anxcloud-cloud-controller-manager test run debug hack go-lint docs-lint docs-lint-fix
