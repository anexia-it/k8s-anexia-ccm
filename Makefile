k8s-anexia-ccm:
	go build

test:
	go run github.com/onsi/ginkgo/v2/ginkgo -p 	\
	    -timeout 0                  			\
	    -race                       			\
	    -coverprofile coverage.out  			\
	    --keep-going                			\
	    ./anx/...
	go tool cover -html=coverage.out -o coverage.html

run: k8s-anexia-ccm
	hack/anxkube-dev-run

debug:
	hack/anxkube-dev-run debug

docs:
	+make -C docs html

versioned-docs:
	+make -C docs versioned-html

depscheck:
	@echo "==> Checking source code dependencies..."
	@go mod tidy
	@git diff --exit-code -- go.mod go.sum || \
		(echo; echo "Found differences in go.mod/go.sum files. Run 'go mod tidy' or revert go.mod/go.sum changes."; exit 1)
	@# reset go.sum to state before checking if it is clean
	@git checkout -q go.sum

fmt:
	gofmt -s -w .

docs-lint:
	@echo "==> Checking docs against linters..."
	@go tool misspell -error -source=text docs/ || (echo; \
		echo "Unexpected misspelling found in docs files."; \
		echo "To automatically fix the misspelling, run 'make docs-lint-fix' and commit the changes."; \
		exit 1)
	@docker run -v $(PWD):/markdown 06kellyjac/markdownlint-cli docs/ || (echo; \
		echo "Unexpected issues found in docs Markdown files."; \
		echo "To apply any automatic fixes, run 'make docs-lint-fix' and commit the changes."; \
		exit 1)

docs-lint-fix:
	@echo "==> Applying automatic docs linter fixes..."
	@go tool misspell -w -source=text docs/
	@docker run -v $(PWD):/markdown 06kellyjac/markdownlint-cli --fix docs/

.PHONY: k8s-anexia-ccm test run debug docs versioned-docs go-lint docs-lint docs-lint-fix fmt
