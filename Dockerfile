# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the shared go-events module (referenced via replace directive)
COPY go-events/ /go-events/

# Copy the source code
COPY . .

# Update replace directive to in-container path and download deps
RUN sed -i 's|=> ../aether-shared/go-events|=> /go-events|' go.mod && \
    go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent-builder ./cmd/main.go

# Build the model-migration tool used by the weekly CronJob
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o model-migrate ./cmd/model-migrate

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binaries from builder stage
COPY --from=builder /app/agent-builder .
COPY --from=builder /app/model-migrate .

# Expose port
EXPOSE 8087

# Command to run
CMD ["./agent-builder"]