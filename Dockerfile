# ✅ Build stage with correct Go version
FROM golang:1.24.3 AS builder
WORKDIR /app
COPY . .

RUN go build -o profilerd ./cmd/profilerd/main.go
RUN go build -o simulator ./cmd/simulator/main.go

# ✅ Final image using Debian slim (glibc compatible)
FROM debian:bullseye-slim
WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/profilerd .
COPY --from=builder /app/simulator .
EXPOSE 8080
CMD ["./profilerd"]


