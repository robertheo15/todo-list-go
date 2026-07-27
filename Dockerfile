FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o main ./cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main /app/main

EXPOSE 8080

CMD ["/app/main"]
