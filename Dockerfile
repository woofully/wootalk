# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy go mod files from backend directory
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source code from backend directory
COPY backend/main.go ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy ca-certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/server .

# Expose port (Render uses PORT environment variable)
EXPOSE 8080

# Run
CMD ["./server"]
