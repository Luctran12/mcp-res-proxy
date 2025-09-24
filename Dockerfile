FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -o server ./cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/server .
COPY .env .
CMD ["./server"]
