# Use the official Golang image as the base image
FROM --platform=$BUILDPLATFORM golang:1.20 as builder

# Set up the working directory
WORKDIR /app

# Copy the Go modules and download the dependencies
COPY go.mod go.sum ./
RUN go mod download
# Copy the source code
COPY . .
ARG TARGETPARCH
RUN if $TARGETPARCH == "arm64"; then apt update && apt install gcc-aarch64-linux-gnu -y; fi
# Build the X-ui binary
RUN if $TARGETPARCH == "arm64"; then GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build -o xui-release-arm64 -v main.go; fi
RUN if $TARGETPARCH == "amd64"; then GOOS=linux GOARCH=amd64 go build -o xui-release-amd64 -v main.go; fi

# Start a new stage using the base image
FROM ubuntu:20.04
# Set up the working directory
WORKDIR /app
# Copy the X-ui binary and required files from the builder stage
RUN arch=$(uname -m); \
    if [ "$arch" = "aarch64" ]; then \
        COPY --from=builder /app/xui-release-arm64 /app/x-ui/xui-release; \
    else \
        COPY --from=builder /app/xui-release-amd64 /app/x-ui/xui-release; \
    fi
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

RUN if [ "$arch" = "aarch64" ]; then \
        wget https://github.com/mhsanaei/xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip \
        && unzip Xray-linux-arm64-v8a.zip \
        && rm -f Xray-linux-arm64-v8a.zip geoip.dat geosite.dat iran.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
        && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
        && wget https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
        && mv xray xray-linux-arm64 \
        fi

RUN if [ "$arch" = "amd64" ]; then \
        wget https://github.com/mhsanaei/Xray-core/releases/latest/download/Xray-linux-64.zip \
 && unzip Xray-linux-64.zip \
 && rm -f Xray-linux-64.zip geoip.dat geosite.dat iran.dat \
 && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
 && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
 && wget https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
 && mv xray xray-linux-amd64 \
 fi

WORKDIR /app
RUN chmod +x /app/x-ui/x-ui.sh

# Set the entrypoint
ENTRYPOINT ["/app/x-ui/xui-release"]
