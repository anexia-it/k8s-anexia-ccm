FROM golang:1.17 as builder
ARG version="v0.0.0-unreleased"
WORKDIR /go/src/github.com/github.com/anexia-it/k8s-anexia-ccm
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o ccm -ldflags "-s -w -X github.com/anexia-it/k8s-anexia-ccm/anx/provider.Version=$version"

FROM alpine:3.15
EXPOSE 8080
RUN apk --no-cache add ca-certificates=20211220-r0
WORKDIR /app/
COPY --from=builder /go/src/github.com/github.com/anexia-it/k8s-anexia-ccm/ccm .
CMD ["/app/ccm", "--cloud-provider=anexia"]
