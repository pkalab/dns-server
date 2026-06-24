FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server/ && \
    CGO_ENABLED=0 go build -o dnscli ./cmd/dns/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata bind-tools
RUN adduser -D -h /app appuser
WORKDIR /app
COPY --from=builder /build/server .
COPY --from=builder /build/dnscli .
COPY config.yaml .
COPY sites/ ./sites/
RUN mkdir -p /app/storage && chown -R appuser:appuser /app
USER appuser
EXPOSE 53/udp 53/tcp 80 443
VOLUME ["/app/storage", "/app/sites"]
ENTRYPOINT ["/app/server"]
CMD ["-config", "/app/config.yaml"]
