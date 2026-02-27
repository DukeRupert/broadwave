FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o broadwave .

FROM alpine:3

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /build/broadwave .

RUN mkdir -p /app/data /app/backups && chown -R appuser:appuser /app

USER appuser

EXPOSE 80

CMD ["./broadwave", "-config", "/app/config.toml"]
