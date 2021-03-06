# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY api/ api/
COPY apis/ apis/
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags '-extldflags "-static"' -o agent cmd/agent/main.go

FROM alpine:3.11 as alpine

RUN mkdir -p /out/etc/apk && cp -r /etc/apk/* /out/etc/apk/

RUN apk -U add --no-cache --initdb -p /out \
  alpine-baselayout \
  ca-certificates \
  util-linux \
  sgdisk \
  coreutils

FROM scratch
WORKDIR /
COPY --from=alpine /out /

COPY --from=builder /workspace/agent .

ENTRYPOINT [ "/agent" ]
CMD []
