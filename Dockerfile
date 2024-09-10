FROM golang:1.23 as builder
ARG version="v0.0.0-unreleased"
WORKDIR /go/src/github.com/github.com/anexia-it/k8s-anexia-ccm
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o ccm -ldflags "-s -w -X github.com/anexia-it/k8s-anexia-ccm/anx/provider.Version=$version"

FROM alpine:3.20.2
EXPOSE 8080

# Hadolint wants us to pin apk packages to specific versions, mostly to make sure sudden incompatible changes
# don't get released - for ca-certificates this only gives us the downside of randomly failing docker builds
# hadolint ignore=DL3018
RUN apk --no-cache add ca-certificates

WORKDIR /app/
COPY --from=builder /go/src/github.com/github.com/anexia-it/k8s-anexia-ccm/ccm .
CMD ["/app/ccm", "--cloud-provider=anexia"]
