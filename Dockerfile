FROM golang:1.17 as builder
WORKDIR /go/src/github.com/github.com/anexia-it/anxcloud-cloud-controller-manager
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o ccm -ldflags '-s -w'

FROM alpine:3.14
EXPOSE 8080
RUN apk --no-cache add ca-certificates=20191127-r5
WORKDIR /app/
COPY --from=builder /go/src/github.com/github.com/anexia-it/anxcloud-cloud-controller-manager/ccm .
CMD ["/app/ccm", "--cloud-provider=anx"]
