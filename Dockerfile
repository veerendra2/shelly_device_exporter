# This Dockerfile is only for development environment
# Release versions use distroless images built via GoReleaser with ko
# See .goreleaser.yml
#
FROM golang:1.26.4 AS builder
WORKDIR /app
RUN curl -sL https://taskfile.dev/install.sh | sh
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN /app/bin/task build

FROM alpine:3.24.1
RUN apk update && apk add --no-cache ca-certificates
WORKDIR /
COPY --from=builder /app/dist/shelly_device_exporter .
USER nobody
ENTRYPOINT ["/shelly_device_exporter"]
