# Use the official Golang image as the base image
FROM --platform=$BUILDPLATFORM golang:1.20 as builder
# Set up the working directory
WORKDIR /app

# Copy the Go modules and download the dependencies
COPY go.mod go.sum ./
RUN go mod download
# Copy the source code
COPY . .
ARG TARGETOS TARGETARCH
# Build the X-ui binary
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o xui-release-$TARGETARCH -v main.go

# Start a new stage using the base image
FROM ubuntu:20.04
# Set up the working directory
WORKDIR /app

# Copy the X-ui binary and required files from the builder stage
COPY --from=builder /app/xui-release-$TARGETARCH /app/x-ui/xui-release
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
RUN wget https://github.com/mhsanaei/Xray-core/releases/latest/download/Xray-linux-64.zip \
 && unzip Xray-linux-64.zip \
 && rm -f Xray-linux-64.zip geoip.dat geosite.dat iran.dat \
 && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
 && wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
 && wget https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
 && arch=(dpkg --print-architecture) \
 && mv xray xray-linux-$arch

WORKDIR /app
RUN chmod +x /app/x-ui/x-ui.sh

# Set the entrypoint
ENTRYPOINT ["/app/x-ui/xui-release"]
