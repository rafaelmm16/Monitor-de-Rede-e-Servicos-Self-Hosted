# Stage 1: Build single static Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /netpulse ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /netpulse /usr/local/bin/netpulse

RUN mkdir -p /app/data
ENV PORT=8080
ENV DB_PATH=/app/data/netpulse.db

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/netpulse"]
