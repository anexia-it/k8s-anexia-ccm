anxcloud-cloud-controller-manager:
	go build

test:
	go test -v                      \
	    -cover                      \
	    -coverprofile=coverage.out  \
	    ./anx/...                && \
	go tool cover                   \
	    -html=coverage.out          \
	    -o coverage.html

run: anxcloud-cloud-controller-manager
	hack/anxkube-dev-run

debug:
	hack/anxkube-dev-run debug

.PHONY: anxcloud-cloud-controller-manager test run debug
