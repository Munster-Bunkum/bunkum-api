# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder
WORKDIR /app

# Download dependencies first (cached layer unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server ./cmd/server

# Final image — just alpine + the compiled binary. No Go toolchain needed.
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server ./server
EXPOSE 8080
CMD ["./server"]
