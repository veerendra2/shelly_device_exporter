FROM golang:1.26.0 AS BUILDER
WORKDIR /app
RUN curl -sL https://taskfile.dev/install.sh | sh
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN /app/bin/task build

FROM alpine:3.23.3
RUN apk update && apk add --no-cache ca-certificates
WORKDIR /
COPY --from=BUILDER /app/dist/your-app-name .
USER nobody
ENTRYPOINT ["/your-app-name"]

# FROM gcr.io/distroless/static-debian13
# WORKDIR /
# COPY --from=BUILDER /app/dist/your-app-name .
# ENTRYPOINT ["/your-app-name"]
