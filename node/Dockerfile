# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk --no-cache add curl unzip

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build node service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o node-service ./node/main.go

# Download XRAY Core based on target architecture
# TARGETARCH is automatically set by Docker BuildKit
ARG TARGETARCH=amd64
ARG TARGETOS=linux
RUN mkdir -p bin && \
    cd bin && \
    case ${TARGETARCH} in \
        amd64) \
            ARCH="64" \
            FNAME="amd64" \
            ;; \
        arm64) \
            ARCH="arm64-v8a" \
            FNAME="arm64" \
            ;; \
        arm) \
            ARCH="arm32-v7a" \
            FNAME="arm32" \
            ;; \
        armv6) \
            ARCH="arm32-v6" \
            FNAME="armv6" \
            ;; \
        386) \
            ARCH="32" \
            FNAME="i386" \
            ;; \
        *) \
            ARCH="64" \
            FNAME="amd64" \
            ;; \
    esac && \
    echo "Downloading Xray for ${TARGETARCH} (ARCH=${ARCH}, FNAME=${FNAME})" && \
    curl -sfLRO "https://github.com/XTLS/Xray-core/releases/download/v25.12.8/Xray-linux-${ARCH}.zip" && \
    echo "Unzipping..." && \
    unzip -q "Xray-linux-${ARCH}.zip" && \
    echo "Files after unzip:" && \
    ls -la && \
    echo "Removing zip and old data files..." && \
    rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat && \
    echo "Renaming xray to xray-linux-${FNAME}..." && \
    mv xray "xray-linux-${FNAME}" && \
    chmod +x "xray-linux-${FNAME}" && \
    echo "Verifying xray binary:" && \
    ls -lh "xray-linux-${FNAME}" && \
    test -f "xray-linux-${FNAME}" && echo "✓ xray-linux-${FNAME} exists" && \
    echo "Downloading geo files..." && \
    curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat && \
    curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat && \
    curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat && \
    curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat && \
    curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat && \
    curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat && \
    echo "Final files in bin:" && \
    ls -lah && \
    echo "File sizes:" && \
    du -h * && \
    cd .. && \
    echo "Verifying files in /build/bin:" && \
    ls -lah /build/bin/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/node-service .

# Copy XRAY binary and data files
# Use wildcard to copy all files from bin directory
COPY --from=builder /build/bin/ ./bin/

# Verify files were copied and make executable
RUN echo "Contents of /app/bin after COPY:" && \
    ls -la ./bin/ && \
    echo "Looking for xray binary..." && \
    if [ -f ./bin/xray-linux-amd64 ]; then \
        chmod +x ./bin/xray-linux-amd64 && \
        echo "✓ Found and made executable: xray-linux-amd64"; \
    elif [ -f ./bin/xray ]; then \
        chmod +x ./bin/xray && \
        mv ./bin/xray ./bin/xray-linux-amd64 && \
        echo "✓ Found xray, renamed to xray-linux-amd64"; \
    else \
        echo "✗ ERROR: No xray binary found!" && \
        echo "All files in bin directory:" && \
        find ./bin -type f -o -type l && \
        exit 1; \
    fi

# Create directories for config and logs
RUN mkdir -p /app/config /app/logs

# Set environment variables for paths
ENV XUI_BIN_FOLDER=/app/bin
ENV XUI_LOG_FOLDER=/app/logs

# Expose API port
EXPOSE 8080

# Run node service
# The API key will be read from NODE_API_KEY environment variable
CMD ["./node-service", "-port", "8080"]
