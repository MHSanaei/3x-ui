# Use the official Golang image as the base image
FROM --platform=$BUILDPLATFORM golang:1.20 as builder
ARG TARGETOS TARGETARCH
# Set up the working directory
WORKDIR /app

# Copy the Go modules and download the dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .


RUN if [ "$TARGETARCH" = "arm64" ]; then apt update && apt install gcc-aarch64-linux-gnu -y; fi

# Build the X-ui binary
RUN if [ "$TARGETARCH" = "arm64" ]; then \
       CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build -o xui-release -v main.go; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
       CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o xui-release -v main.go; \
    fi

# Start a new stage using the base image
FROM ubuntu:20.04

# Set up the working directory
WORKDIR /app

# Copy the X-ui binary and required files from the builder stage
COPY --from=builder /app/xui-release /app/x-ui/xui-release
COPY x-ui.service /app/x-ui/x-ui.service
COPY x-ui.sh /app/x-ui/x-ui.sh

# Set up the runtime environment
RUN apt-get update && apt-get install -y \
    wget \
    unzip \
    tzdata \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app/x-ui/bin

# Download and set up the required files
RUN arch=$(uname -m) && \
    if [ "$arch" = "aarch64" ]; then \
        wget https://github.com/mhsanaei/xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip \
        && unzip Xray-linux-arm64-v8a.zip \
        && rm -f Xray-linux-arm64-v8a.zip geoip.dat geosite.dat iran.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
        && wget https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
        && mv xray xray-linux-arm64; \
    elif [ "$arch" = "amd64" ]; then \
        wget https://github.com/mhsanaei/Xray-core/releases/latest/download/Xray-linux-64.zip \
        && unzip Xray-linux-64.zip \
        && rm -f Xray-linux-64.zip geoip.dat geosite.dat iran.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
        && wget https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
        && mv xray xray-linux-amd64; \
    fi

WORKDIR /app
RUN chmod +x /app/x-ui/x-ui.sh

# Set the entrypoint
ENTRYPOINT ["/app/x-ui/xui-release"]
