# Multi-stage build for HomevoltScraper with headless Chromium

# Build stage
FROM docker.io/golang:1.24 AS builder
WORKDIR /app

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static Linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/homevoltscraper ./cmd/homevoltscraper

# Runtime stage with headless Chromium
FROM docker.io/debian:bookworm-slim

# Install Chromium and CA certificates
RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=builder /out/homevoltscraper /usr/local/bin/homevoltscraper

# Default to UTC time
ENV TZ=UTC

# Run the scraper (flags provided at runtime)
ENTRYPOINT ["/usr/local/bin/homevoltscraper"]
