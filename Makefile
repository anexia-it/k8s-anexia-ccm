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

.PHONY: regen-mocks verify-mocks

regen-mocks:
	@echo "==> Installing mock tools (mockgen)"
	@export PATH="$(shell go env GOPATH)/bin:$$PATH"; \
		go install github.com/golang/mock/mockgen@latest
	@echo "==> Regenerating GoMock mocks"
	@export PATH="$(shell go env GOPATH)/bin:$$PATH"; \
		mockgen -package legacyapimock -destination anx/provider/test/legacyapimock/ipam_address.go -mock_names API=MockIPAMAddressAPI go.anx.io/go-anxcloud/pkg/ipam/address API; \
		mockgen -package legacyapimock -destination anx/provider/test/legacyapimock/ipam_prefix.go -mock_names API=MockIPAMPrefixAPI go.anx.io/go-anxcloud/pkg/ipam/prefix API; \
		mockgen -package legacyapimock -destination anx/provider/test/legacyapimock/ipam.go -mock_names API=MockIPAMAPI go.anx.io/go-anxcloud/pkg/ipam API; \
		mockgen -package apimock -destination anx/provider/test/apimock/api_mock.go go.anx.io/go-anxcloud/pkg/api/types API

verify-mocks: regen-mocks
	@echo "==> Verifying generated mocks are committed"
	@git diff --exit-code -- \
		anx/provider/test/legacyapimock \
		anx/provider/test/apimock \
		anx/provider/test/gomockapi \
		anx/provider/test/mockvsphere \
		anx/provider/test/mocklbaas \
		anx/provider/test/mockclouddns \
		anx/provider/test/mockvlan \
		anx/provider/test/mocktest || (echo "Generated mocks differ, run 'make regen-mocks' and commit changes" && exit 1)
