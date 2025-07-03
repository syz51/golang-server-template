# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

ENV GOCACHE=/root/.cache/go-build

# Copy source code
COPY . .

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target="/root/.cache/go-build" \
    CGO_ENABLED=0 GOOS=linux go build -a -o server cmd/server/main.go

# Final stage
FROM gcr.io/distroless/static-debian12:nonroot

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/server .

# Copy config directory
COPY --from=builder /app/configs ./configs

# Expose port
EXPOSE 8080

# Command to run
CMD ["/app/server"] 